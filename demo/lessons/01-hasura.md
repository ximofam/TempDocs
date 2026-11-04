# Bài 1 — Hasura biến bảng PostgreSQL thành GraphQL API

Mục tiêu: hiểu table tracking, query/mutation/subscription và quyền theo dòng/cột. Hasura đọc schema PostgreSQL để sinh GraphQL; bạn không cần viết Go handler CRUD cho từng bảng.

## 1. Chạy và xem dữ liệu

Từ thư mục `demo/`, hoàn thành `make up` và `make bootstrap` trong README, rồi:

```sh
docker compose run --rm demo graphql alice examples/list.graphql
docker compose run --rm demo graphql bob examples/list.graphql
```

Alice chỉ thấy ws-a, Bob chỉ thấy ws-b. Dữ liệu seed có hồ sơ mua vòng bi và mua dầu máy. GraphQL cho phép chọn chính xác trường cần đọc; `work_item { title }` đi theo quan hệ khóa ngoại tới công việc chung.

Đọc [metadata](../hasura/metadata.json):

- `sources` kết nối tới PostgreSQL qua `PG_DATABASE_URL`.
- `tables` khai báo bảng được track. Tạo bảng SQL chưa đủ để bảng tự xuất hiện trên API.
- `object_relationships` đặt tên quan hệ `work_item`.
- `select_permissions.filter` ràng buộc `workspace_id` với `X-Hasura-Workspace-Id`.
- `columns` quy định trường nào role được đọc/sửa.

Role `employee` và `manager` là role Hasura, không phải DB login. Hasura dùng một tài khoản DB runtime rồi áp dụng metadata khi dựng query.

## 2. Thử bằng Hasura Console

Mở http://localhost:8080, nhập `HASURA_ADMIN_SECRET` từ `.env` để mở Console. Trong API Explorer, **tắt header `x-hasura-admin-secret` trước khi thử quyền người dùng**. Lấy JWT:

```sh
docker compose run --rm demo token alice
```

Thêm header `Authorization: Bearer <JWT-vừa-in>` rồi dán nội dung [list.graphql](../examples/list.graphql). Không để admin secret và JWT cùng bật: admin secret có thể bỏ qua giới hạn role, làm bài kiểm tra quyền sai ý nghĩa.

Thử truy vấn chéo workspace:

```graphql
query {
  procurement_requests(where: {workspace_id: {_eq: "ws-b"}}) {
    id description
  }
}
```

Với token Alice, kết quả là mảng rỗng. JWT hết hạn sau một giờ; tạo lại bằng lệnh `token` nếu cần. Không chuyển danh tính bằng cách tự sửa payload token: chữ ký sẽ không hợp lệ.

## 3. Tạo và sửa bản nháp

```sh
docker compose run --rm demo graphql alice examples/create.graphql
```

Ghi lại `id` trả về. Hồ sơ có `status: DRAFT`, `version: 1`, `created_by: alice`, `workspace_id: ws-a`. Bạn không cần truyền hai trường cuối vì metadata dùng column presets lấy từ session JWT. Work item A đã được seed để tập trung học hồ sơ mua sắm; cũng có thể tạo công việc qua mutation `insert_core_work_items_one` với `title`.

Trong Console, thay `<ID>` bằng ID vừa tạo:

```graphql
mutation {
  update_procurement_requests_by_pk(
    pk_columns: {id: "<ID>"}
    _set: {description: "Mua 2 vòng bi", amount: 750000}
  ) { id description amount version }
}
```

Kết quả `version: 2`. Trigger `guard_draft` trong [SQL](../db/001-init.sql) tăng version khi nội dung đổi. `amount` phải dương: thử `-1` để thấy constraint từ DB. Work item thuộc workspace B không thể gắn vào hồ sơ A vì khóa ngoại ghép `(workspace_id, work_item_id)`.

```sh
docker compose run --rm demo graphql alice examples/forbidden-status.graphql
```

Lệnh này **phải lỗi** vì role không có quyền cập nhật `status`. Tương tự, không được tự ghi `version`, `workspace_id` hay `created_by`. `manager` trong demo chỉ đọc qua Hasura, quyết định duyệt qua API Go ở bài 3.

## 4. Quan sát subscription

Dùng token Alice trong API Explorer:

```graphql
subscription {
  procurement_requests(order_by: {id: asc}) {
    id description status version
  }
}
```

Mở cửa sổ khác sửa bản nháp hoặc thực hiện bài 3: kết quả subscription tự cập nhật. Hasura giữ WebSocket để gửi kết quả mới, không cần frontend liên tục gọi GET.

CLI Go cũng có ví dụ kết nối WebSocket:

```sh
docker compose run --rm demo subscribe alice
```

CLI xem trong tối đa 30 giây rồi kết thúc vì deadline; dùng Ctrl+C để dừng sớm. Đọc [subscription.go](../cmd/demo/subscription.go) để xem handshake `connection_init`, `connection_ack`, `subscribe`, `next` của giao thức `graphql-transport-ws`. Mỗi subscription vẫn bị lọc theo workspace.

## 5. Từ thao tác tay tới cấu hình trong repo

Console → Data → bảng → Permissions giúp xem role/filter/column. Chỉ bật admin secret khi quản trị, sau đó tắt để kiểm tra user. Sau khi thử chỉnh quyền, export metadata từ Console để so sánh với file repo; `make bootstrap` sẽ phục hồi cấu hình trong repo và giữ nguyên dữ liệu.

**Bạn đã học được:** SQL giữ dữ liệu/constraint, metadata định nghĩa API/quyền, JWT cung cấp danh tính. Chuyển sang [bài 2](02-kestra.md) để chạy workflow độc lập.
