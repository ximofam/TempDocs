# Bài 2 — Kestra gọi API Go và thử lại khi lỗi

Mục tiêu: đọc được flow YAML, truyền input, xem task logs và phân biệt lỗi tạm thời với execution thất bại.

## 1. Xem flow trong UI

Mở http://localhost:8082, đăng nhập với `KESTRA_USER` / `KESTRA_PASSWORD` trong `.env`. Chọn Flows → namespace `dxos.demo` → `retry-http`. Mã gốc là [retry.yaml](../kestra/retry.yaml), đã được bootstrap import. Bạn cũng có thể tạo/import flow bằng UI; cùng namespace/ID là cùng flow. Nếu sửa trong UI rồi chạy bootstrap, file trong repo được áp dụng lại.

- `namespace` nhóm workflow.
- `id` định danh flow trong namespace.
- `inputs` là dữ liệu nhận khi chạy.
- `tasks` là các bước theo thứ tự.
- Task `call_go` dùng plugin HTTP có sẵn, không chạy chương trình Python.
- Task `show_result` in phản hồi Go để bạn xem output.

## 2. Thành công sau hai lỗi

```sh
docker compose run --rm demo retry 2
```

CLI tạo `run_id` mới, yêu cầu endpoint Go lỗi đúng hai lần đầu, sau đó thành công. Trong Kestra Executions, chọn ID được in ra; xem Gantt, task attempts, logs và Outputs.

Có thể chạy bằng nút Execute trong UI với `run_id: practice-001`, `failures: 2`. Mỗi lần thực hành nên dùng run_id mới, vì Go ghi số lần đã gọi theo run_id. Restart API xóa bộ đếm này; bộ đếm chỉ minh họa retry, không dùng bảo vệ nghiệp vụ.

Đọc [server.go](../internal/server/server.go), endpoint `POST /internal/practice/{run}?failures=N`: trả HTTP 503 trong N lần đầu, rồi HTTP 200 với `attempt` và `ok`. Không có lỗi ngẫu nhiên.

Trong flow, `retry` ở cấp task đặt khoảng chờ 2 giây và `maxAttempts: 3`. `warningOnRetry: false` giúp execution phục hồi thành SUCCESS. HTTP client retry nội bộ được giới hạn một attempt để không che mất số lần task thực hiện. Xem số attempt thực tế trong UI thay vì suy ra số lần gọi từ thời gian chạy.

## 3. Lỗi kéo dài

```sh
docker compose run --rm demo retry 10
```

Lệnh **phải thoát lỗi**, CLI in execution ID và trạng thái FAILED khi hết số lần thử. Task `show_result` không được chạy như một kết quả thành công. Từ execution, mở logs của `call_go` để xem HTTP 503 và các attempt.

Kestra phù hợp với lỗi kết nối hoặc dịch vụ tạm thời không sẵn sàng. Với sai quyền, dữ liệu không hợp lệ hoặc phiên bản cũ, cần sửa nguyên nhân; retry không tự biến một yêu cầu sai thành đúng.

## 4. Secret, timeout và ranh giới

Flow lấy `secret('SERVICE_TOKEN')` cho header Authorization. Compose nhận token thuần từ `.env`, entrypoint tự base64-encode thành `SECRET_SERVICE_TOKEN` trước khi khởi động Kestra; `secret()` đọc và giải mã. Không in token trong task Log. Base64 không bảo vệ bí mật khi ai đó đã có quyền đọc môi trường container.

`timeout` giới hạn task và `options.timeout.readIdleTimeout` giới hạn chờ HTTP. Task có số retry hữu hạn để không treo mãi.

Bài này không tạo hồ sơ, không chuyển trạng thái và không giả lập quyết định duyệt. Chuyển sang [bài 3](03-integration.md) để xem một tác vụ có tác động nghiệp vụ cần được bảo vệ thế nào khi chạy lại.
