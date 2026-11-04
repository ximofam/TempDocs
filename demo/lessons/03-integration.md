# Bài 3 — Hasura → Kestra → API Go

Mục tiêu: nối CRUD với workflow, vẫn giữ quyền, phiên bản và tính lũy đẳng ở nơi thực thi nghiệp vụ.

## 1. Tạo bản nháp

```sh
docker compose run --rm demo graphql alice examples/create.graphql
```

Ghi lại ID. Các lệnh dưới dùng `<ID>` như chỗ cần thay, không chạy nguyên dấu ngoặc. Hồ sơ mới có version 1. Nếu đã sửa nội dung trong bài 1 thì dùng version mới nhận được.

## 2. Gửi duyệt qua Go

```sh
docker compose run --rm demo submit alice <ID> 1
```

Phản hồi: `status: PENDING_APPROVAL`, `version: 2`. Go xác minh JWT, workspace, người tạo, trạng thái và expected version. Trong một transaction, Go chuyển trạng thái và chèn hàng vào `core.outbox`.

Đọc [store.go](../internal/store/store.go), hàm `Submit`. Khóa `FOR UPDATE` ngăn hai lệnh đồng thời cùng dùng một phiên bản. DB vẫn kiểm tra số tiền dương và khóa ngoại. Trạng thái đang gửi duyệt không cho sửa nội dung; muốn hỗ trợ sửa/gửi lại cần thêm vòng đời rõ ràng, ngoài phạm vi demo này.

## 3. Theo dõi sự kiện và workflow

Hasura Console → Events → `request_submitted` → tìm event INSERT mới → Delivery logs. Khi cần xem bảng outbox trong Data, dùng quyền quản trị; nhân viên không có API đọc bảng outbox.

Hasura gửi payload tới webhook Kestra có dạng:

```json
{
  "event": {
    "data": {
      "new": {
        "event_id": "UUID-của-outbox",
        "event_type": "request.submitted",
        "workspace_id": "ws-a",
        "request_id": "UUID-của-hồ-sơ",
        "version": 2
      }
    }
  }
}
```

Payload thật có thêm các trường envelope và `created_at`. Flow chỉ lấy các trường cần cho callback. Hasura envelope event ID và `event_id` của outbox là hai ID khác nhau; khóa chống trùng dùng ID nghiệp vụ trong outbox để vẫn ổn định khi replay.

Kestra UI → Executions → flow `procurement-submitted` → task `create_inbox`. [Flow YAML](../kestra/procurement.yaml) gọi `POST /internal/events` bằng service token. Go đối chiếu toàn bộ event với outbox đã lưu, kiểm trạng thái/phiên bản, rồi tạo mục hộp duyệt và ghi `processed_events` trong cùng transaction.

Xem mục chờ duyệt bằng token `manager-a` trong Hasura API Explorer:

```graphql
query {
  procurement_approval_inbox {
    event_id request_id version workspace_id
  }
}
```

Go không tin workspace trong payload để tự cấp quyền. Callback chỉ chấp nhận event đã có trong DB. Webhook key chỉ cho phép tạo execution; service token cho phép gọi endpoint nội bộ; JWT người dùng không thay thế được hai cơ chế này.

## 4. Người quản lý quyết định

Sau khi inbox xuất hiện:

```sh
docker compose run --rm demo approve manager-a <ID> 2
```

Kết quả APPROVED, version 3. Hasura query/subscription thấy trạng thái mới ngay khi transaction hoàn tất. Kestra không tự quyết định duyệt.

Với một hồ sơ mới khác, thử `reject manager-a <ID> 2` để thấy REJECTED. Demo lưu trạng thái quyết định; chưa có biểu mẫu lý do từ chối, sửa/gửi lại hoặc audit đầy đủ.

Các tình huống cố ý lỗi:

```sh
docker compose run --rm demo approve alice <ID> 2
docker compose run --rm demo approve manager-b <ID> 2
docker compose run --rm demo approve manager-a <ID> 1
```

Alice không có role duyệt; manager-b không thuộc workspace A; phiên bản 1 đã cũ. API lần lượt từ chối, không đổi dữ liệu. Nếu đã approve thì gọi lại với version 2 cũng bị xung đột trạng thái/phiên bản. Việc chấp nhận replay event không có nghĩa chấp nhận thực hiện lại lệnh phê duyệt.

## 5. Gửi lại cùng sự kiện

```sh
docker compose run --rm demo replay <ID>
```

CLI quản trị local đọc event từ DB và gửi lại envelope tới webhook Kestra. Execution mới phải hoàn tất, nhưng inbox không có dòng trùng. Hàm `Deliver` dùng advisory lock theo event ID trong transaction, kiểm `processed_events`, trả lại kết quả cũ khi đã xử lý. Unique constraint là lớp bảo vệ bổ sung, không chỉ dựa vào kiểm tra bằng Go trước khi insert.

Nếu event chưa xử lý nhưng phiên bản hồ sơ đã cũ, kết quả là `skipped_stale` và không tạo inbox. Không cần tự sửa DB để thử điều này: `make test` có kiểm thử trường hợp đó, cùng tám callback đồng thời.

## 6. Thấy lỗi và phục hồi

Có hai vòng retry riêng:

- Hasura retry giao webhook khi không nhận được HTTP thành công.
- Kestra retry task callback khi Go lỗi; nhận webhook thành công không bảo đảm task đã thành công.

Để thử lỗi kết nối, có thể dừng API **sau khi đã submit thành công, trước khi replay**; dùng lại ID hồ sơ đã gửi duyệt ở các bước trên:

```sh
docker compose stop api
docker compose run --rm demo replay <ID>
```

CLI sẽ báo execution FAILED sau khi task hết lượt thử. Quan sát logs trong Kestra, rồi phục hồi:

```sh
docker compose start api
docker compose run --rm demo replay <ID>
```

CLI replay chỉ cần DB để lấy event và Kestra để gửi webhook; callback mới cần API. Execution lần đầu vẫn được giữ ở trạng thái FAILED để điều tra; replay tạo execution khác. Không có bộ quét tự động khôi phục mọi execution lỗi trong demo.

Cuối cùng chạy `make smoke`: chương trình tự tạo hồ sơ riêng, kiểm quyền/JWT/phiên bản, chờ đường Event Trigger thật tạo inbox, duyệt, replay và chạy bài retry. Đọc [smoke.go](../cmd/demo/smoke.go) như một ví dụ client tích hợp bằng Go.
