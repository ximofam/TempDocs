# Kết quả kiểm chứng — 2026-11-04

Đã chạy dịch vụ thật bằng Docker trên Linux:

- PostgreSQL `17.6-alpine`, Hasura `v2.51.3`, Kestra `v2.0.8`.
- Build container bằng Go `1.26.1-alpine`; kiểm tra host bằng Go `1.27.3`.
- `docker compose config --quiet`: thành công.
- `docker compose run --rm test`: thành công, gồm test PostgreSQL thật và test bằng DB role `api_runtime`.
- `go test -race ./...`, `go vet ./...`, `go build -buildvcs=false ./...`: thành công. Test DB trên host skip nếu thiếu URL; test DB đầy đủ đã chạy trong container.
- Bootstrap chạy lặp lại thành công, giữ dữ liệu.
- Smoke xuyên suốt trên stack hiện tại và một stack thử độc lập với volume ban đầu trống: `SMOKE PASS`.
- `retry 2`: execution SUCCESS; `retry 10`: execution FAILED và CLI thoát lỗi đúng mong đợi.
- Dừng API → replay event đã submit: execution FAILED; khởi động lại API → replay: SUCCESS.
- Subscription nhận hai thông điệp `next`: snapshot đầu và cập nhật sau mutation thật.
- Kiểm tra liên kết Markdown, SPDX, LF và ảnh tham chiếu cũ: thành công. Hướng dẫn UI dựa trên API đã chạy; không có ảnh chụp kiểm tra giao diện bằng trình duyệt.

Các kiểm thử bao gồm JWT hết hạn/sai chữ ký, cô lập workspace, khóa ngoại khác workspace, CRUD không được ghi status, người duyệt sai quyền, phiên bản cũ, tám callback đồng thời, sự kiện cũ và replay sau khi phê duyệt.

## Điều chỉnh so với kế hoạch

- Kestra 2.0.8 cần Basic Auth cho UI/API. Bootstrap/CLI gửi Basic Auth đúng service; webhook vẫn dùng key riêng.
- Import YAML dùng API namespace `/flows/dxos.demo?delete=false`. `/flows/import` yêu cầu multipart, không nhận YAML thô theo cách ban đầu dự kiến.
- HTTP options dùng `timeout.readIdleTimeout`; retry nội bộ cần `type`. Cấu hình được validator và execution thật xác nhận.
- Hasura cần header JWT trong `connection_init.payload.headers`; client xử lý ping/pong cả trước ack.
- Trigger Hasura thực thi theo DB role tạo outbox. Sau metadata, bootstrap cấp `USAGE` schema `hdb_catalog` và `INSERT` đúng năm cột event log cho API. Không đổi hàm generated vì Hasura có thể tạo lại hàm khi reload.
- Callback dùng transaction advisory lock theo event ID thay vì khóa hàng outbox. Không cần cấp UPDATE trên outbox. Unique constraint và transaction vẫn bảo vệ tác động trùng.
- API runtime không nhận admin secret Hasura hoặc tài khoản quản trị Kestra; các giá trị này chỉ có trong CLI công cụ local.

Có một vòng rà soát mã độc lập; đã sửa fixture stale event để không đua với workflow đang chạy và làm rõ replay cần hồ sơ đã submit.
