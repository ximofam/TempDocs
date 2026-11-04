# Kế hoạch triển khai demo Hasura và Kestra bằng Go

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans nếu thực hiện trực tiếp, hoặc superpowers:subagent-driven-development nếu người dùng chọn chia agent. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Tạo ba bài thực hành tiếng Việt chạy được trong `demo/`, từ Hasura CRUD đến Kestra retry và tích hợp mua sắm.

**Architecture:** Một Docker Compose chạy PostgreSQL, Hasura, Kestra và API Go. CLI Go phát JWT demo, gọi GraphQL/API và chạy smoke test; trạng thái nghiệp vụ chỉ đổi qua API lệnh. Outbox, phiên bản và unique constraint bảo vệ callback trước sự kiện trùng/cũ.

**Tech Stack:** Go 1.26+, `net/http`, `database/sql`, `pgx/v5/stdlib`, `golang-jwt/jwt/v5`; PostgreSQL 17; Hasura `v2.51.3`; Kestra `v2.0.8`; Docker Compose v2. Ghim patch của PostgreSQL và Go image được kiểm tra tồn tại khi triển khai; lưu `go.sum`.

**Spec:** [Thiết kế đã duyệt](../specs/2026-11-03-hasura-kestra-demo-design.md).

## Global Constraints

- Mọi mã nguồn, cấu hình, dữ liệu mẫu và hướng dẫn demo nằm trong `demo/`.
- Dùng UTF-8, LF và SPDX Apache-2.0 cho file mã nguồn tự viết.
- Bám `core.work_items` và bảng nghiệp vụ riêng; không sửa tài liệu kiến trúc gốc.
- Chỉ mở cổng localhost; không dùng tag `latest`, cloud hoặc dịch vụ trả phí.
- Không cần Go trên host nếu dùng container; không cần Kubernetes, Operator, Keycloak, AI hay UI riêng.
- CRUD không sửa trạng thái hoặc phiên bản; workflow không phê duyệt thay người dùng.
- Bootstrap lặp lại giữ dữ liệu; reset là lệnh riêng ghi rõ phạm vi xóa.
- Checkout hiện không có Git, bỏ qua commit và không tự tạo repository.

## Review Focus

1. JWT sai chữ ký, hết hạn hoặc claim thiếu phải bị từ chối ở Hasura và Go.
2. Hồ sơ và công việc khác workspace không thể liên kết hoặc được duyệt bằng cách đổi ID.
3. Hai callback đồng thời cùng sự kiện chỉ tạo một mục hộp duyệt.
4. Hồ sơ sửa bản nháp hoặc gửi lại làm phiên bản tăng; kết quả phiên bản cũ không được áp dụng.
5. Kestra đã nhận webhook nhưng execution thất bại phải hiện lỗi rõ, có cách replay, không báo nghiệp vụ hoàn tất.

## Cấu trúc file

- `demo/compose.yaml`, `.env.example`, `Dockerfile`, `Makefile`, `.gitignore`: môi trường, build, lệnh khởi động và kiểm thử.
- `demo/db/001-init.sql`: database, roles, schema, constraints, trigger phiên bản và seed lũy đẳng.
- `demo/hasura/metadata.json`: bảng, quan hệ, quyền và Event Trigger.
- `demo/kestra/retry.yaml`, `procurement.yaml`: hai workflow HTTP.
- `demo/go.mod`, `go.sum`: phụ thuộc.
- `demo/internal/auth/{token.go,token_test.go}`: danh tính demo và JWT.
- `demo/internal/store/{store.go,store_test.go}`: giao dịch nghiệp vụ và kiểm thử DB thật.
- `demo/internal/server/{server.go,server_test.go}`: HTTP, xác thực và endpoint retry.
- `demo/internal/bootstrap/bootstrap.go`: readiness hữu hạn, nạp metadata/flows.
- `demo/cmd/api/main.go`: server.
- `demo/cmd/demo/main.go`: CLI token, request, bootstrap, retry, replay và smoke.
- `demo/README.md`, `lessons/01-hasura.md`, `02-kestra.md`, `03-integration.md`: hướng dẫn thực hành.
- `AGENTS.md`: thêm setup và lệnh kiểm thử demo.

## Task 1: Hasura, dữ liệu và JWT

**Interfaces:** `auth.Identity{UserID, WorkspaceID, Role string}`; `auth.Sign(identity, ttl) (string,error)` và `auth.Verify(token) (Identity,error)`, dùng HS256, issuer `dxos-demo`, hạn token và role trong allowlist. CLI `token <alice|bob|manager-a|manager-b>` phát token dùng cho Hasura và Go. Alice/manager-a thuộc workspace A; bob/manager-b thuộc B.

- [x] Viết `TestVerifyRejectsExpiredForgedAndMissingClaims`: token hợp lệ trả đúng identity; hết hạn, chữ ký sai, thuật toán sai, thiếu claim hoặc role lạ đều lỗi.
- [x] Chạy `cd demo && go test ./internal/auth` trước implementation, ghi nhận fail.
- [x] Tạo module và triển khai auth; khóa local qua env, không nhận identity tùy ý qua HTTP.
- [x] Tạo Compose, Dockerfile multi-stage, `.env.example`, Makefile và schema. Cổng mặc định Hasura 8080, Kestra 8082, API 8090; DB chỉ trong mạng Compose. PostgreSQL tách database và tài khoản Kestra, Hasura metadata và ứng dụng; Go runtime không dùng superuser. Có volume dữ liệu.
- [x] Schema: `core.work_items`, `procurement.requests`, `procurement.approval_inbox`, `core.outbox`, `core.processed_events`; ID UUID, workspace text, FK ghép workspace/ID, amount > 0, version integer tăng từ 1. Trigger khóa sửa nội dung ngoài DRAFT, tăng version khi nội dung đổi; lệnh chuyển trạng thái cũng tăng đúng một lần.
- [x] Seed bằng khóa cố định và `ON CONFLICT DO NOTHING`; khởi tạo schema qua init SQL. Metadata track quan hệ, select theo workspace; insert preset người tạo/workspace, update chỉ bản nháp của chính mình và các cột nội dung. Nhân viên không có quyền ghi status/version/outbox/inbox.
- [x] CLI cung cấp `graphql <identity> <file>` và `bootstrap`; bootstrap replace metadata cho riêng demo, kiểm tra inconsistent metadata, không đụng bản ghi nghiệp vụ. Chưa tạo Event Trigger tới khi flow đã được nạp ở Task 3.
- [x] Chạy auth tests và kiểm tra Compose; thử query/mutation bằng JWT, chữ ký sai, workspace khác và sửa status. Ghi assertion tương ứng trong smoke CLI để tái chạy ở Task 4.

## Task 2: API lệnh và chống tác động trùng

**Interfaces:** `store.Submit(ctx, identity, requestID, expectedVersion)`; `store.Decide(ctx, identity, requestID, expectedVersion, decision)`; `store.Deliver(ctx, Event)`. Event gồm `event_id`, `event_type=request.submitted`, `workspace_id`, `request_id`, `version`. Các phương thức trả kết quả JSON có ID/status/version; lỗi phân loại 403, 404, 409, 422.

- [x] Viết DB tests chạy với `TEST_DATABASE_URL`: `TestSubmitOwnershipAndWorkspace`, `TestDecideRequiresManagerAndDifferentAuthor`, `TestStaleVersion`, `TestConcurrentDeliverCreatesOneInbox`, `TestOldUnprocessedEventIsSkipped`, `TestCrossWorkspaceForeignKey`, `TestDraftEditIncrementsVersion`. Test phải dùng PostgreSQL thật, bỏ qua rõ ràng nếu chưa cung cấp URL.
- [x] Chạy tests trước implementation để ghi nhận fail; sau đó triển khai store với transaction, khóa bản ghi và constraint. Submit tạo outbox cùng transaction; approve/reject cần inbox đúng phiên bản. `processed_events` và tác động inbox ghi cùng transaction; callback đã xử lý trả kết quả cũ. Không tin event payload: đối chiếu outbox trong DB với toàn bộ trường.
- [x] Viết HTTP tests cho JWT thiếu/sai, payload JSON lỗi, service credential sai và timeout; chạy fail.
- [x] Triển khai `GET /healthz`, `POST /requests/{id}/submit`, `/approve`, `/reject` với body `expected_version`; `POST /internal/events` dùng Bearer service token riêng và so sánh constant-time. Endpoint nội bộ không chấp nhận JWT nhân viên làm service token. Dùng timeout, body limit và error JSON rõ ràng.
- [x] Thêm CLI `submit`, `approve`, `reject`, `replay` gọi API; replay là thao tác học local lấy event đã có và tái gửi webhook, không tạo event giả từ người dùng cuối.
- [x] Chạy unit + DB tests, `go test -race ./...`, `go vet ./...`; thử callback đồng thời và version cũ trên stack.

## Task 3: Workflow Kestra và tích hợp Event Trigger

**Interfaces:** flow namespace `dxos.demo`, ID `retry-http` và `procurement-submitted`. Webhook `/api/v1/main/executions/webhook/dxos.demo/procurement-submitted/<key>`; dữ liệu Hasura nằm ở `trigger.body.event.data.new` theo cấu trúc payload thực tế đã kiểm tra. Callback gọi `http://api:8090/internal/events`.

- [x] Đọc tài liệu HTTP Request, retry và webhook của Kestra 2.0.8; đối chiếu API authentication/bootstrap với image ghim. Dùng HTTP tasks, không mount Docker socket và không cần Python.
- [x] Triển khai endpoint có service auth `POST /internal/practice/{runID}?failures=N`, giữ số lần gọi theo runID trong bộ nhớ; N giới hạn 0–10, fail 503 đúng N lần rồi trả 200; restart API reset trạng thái. Tách khỏi mọi giao dịch nghiệp vụ.
- [x] Viết HTTP test chứng minh runID độc lập và lỗi đúng số lần, N không hợp lệ bị từ chối. Chạy fail rồi triển khai.
- [x] Viết `retry.yaml` có input runID/failures, HTTP timeout, retry constant interval 2 giây, maxAttempt 3; kiểm chứng ý nghĩa maxAttempt trên engine thật. Flow thành công sau lỗi ngắn và failed khi lỗi kéo dài.
- [x] Viết `procurement.yaml` nhận webhook có key, gọi Go bằng HTTP task với service token cấu hình backend; callback body chỉ chuyển trường của outbox. Cấu hình webhook key/token qua cơ chế env hoặc template bootstrap được bản Kestra hỗ trợ, không ghi credential vào log task.
- [x] Bootstrap đợi readiness có timeout, nạp flow bằng API create/update, kiểm tra lỗi; sau đó tạo Event Trigger outbox INSERT với webhook URL từ env, retry hữu hạn. Chạy lại hai lần không tạo thêm flow hoặc bản ghi. Đảm bảo trigger không bật trước flow.
- [x] CLI `retry` kích hoạt execution và `replay` gọi lại webhook. Poll trạng thái execution với deadline; HTTP 2xx nhận webhook không được xem là kết quả hoàn thành workflow.
- [x] Thực hiện submit thật, xem Hasura delivery logs và Kestra execution, chờ inbox. Replay tạo execution mới nhưng inbox không tăng. Gây callback lỗi tạm thời và lỗi kéo dài, xem retry/failed rồi phục hồi và replay.

## Task 4: Bài học, smoke test và bàn giao

**Interfaces:** `docker compose run --rm demo bootstrap`, `docker compose run --rm demo smoke`; Makefile bọc `up`, `bootstrap`, `test`, `smoke`, `down`, `reset`. Reset yêu cầu người chạy xác nhận rõ trước `down -v`.

- [x] CLI smoke dùng dữ liệu UUID riêng mỗi lần chạy: tạo draft qua GraphQL, query/subscription, sửa draft tăng version, submit qua Go, chờ inbox, approve bằng manager và kiểm status; replay và kiểm unique inbox. Kiểm sai chữ ký/hết hạn, đọc workspace khác, FK workspace khác, sửa status, sai quyền và expected version cũ. GraphQL HTTP 200 có `errors` vẫn là lỗi khi request mong thành công.
- [x] Test subscription dùng Go WebSocket client theo giao thức Hasura hỗ trợ; timeout hữu hạn và đóng kết nối sạch. Không để phụ thuộc này chặn các bài CRUD hoặc workflow khi chạy riêng.
- [x] Viết README và ba bài học: vai trò công cụ, URL/login Console, lệnh copy được, file cần đọc, kết quả mong đợi, tình huống lỗi, quan hệ Hasura role với DB role, phạm vi JWT local, khác biệt webhook accepted/execution succeeded, cách replay và dọn dữ liệu. Có bài nhập flow/chạy query bằng UI bên cạnh CLI tự động.
- [x] Hướng dẫn export metadata sau khi thử chỉnh Console; bootstrap dùng metadata trong repo là nguồn chuẩn cho demo. Giải thích biến môi trường credential chỉ dùng local và không dùng admin secret khi học quyền.
- [x] Cập nhật AGENTS.md với đường dẫn demo, setup, tests và reset; không thay nội dung hướng dẫn proposal.
- [x] Kiểm tra `docker compose config --quiet`, `go test ./...`, `go vet ./...`, tests DB và race; chạy bootstrap hai lần và smoke trên dịch vụ thật. Kiểm tra links, SPDX, LF và ảnh tham chiếu cũ còn tồn tại.
- [x] Báo kết quả thực tế, phiên bản image đã dùng, tài nguyên đã quan sát và mọi phần không chạy được do môi trường. Không tuyên bố stack đã chạy nếu chỉ kiểm tra cấu hình.

## Tài liệu chính thức đã đối chiếu

- [Hasura Docker](https://hasura.io/docs/2.0/deployment/deployment-guides/docker/), [JWT](https://hasura.io/docs/2.0/auth/authentication/jwt/), [Event Triggers](https://hasura.io/docs/2.0/event-triggers/create-trigger/).
- [Hasura v2.51.3](https://github.com/hasura/graphql-engine/releases/tag/v2.51.3).
- [Kestra v2.0.8](https://github.com/kestra-io/kestra/releases/tag/v2.0.8), [Webhook](https://kestra.io/docs/workflow-components/triggers/webhook-trigger), [Docker Compose mẫu](https://raw.githubusercontent.com/kestra-io/kestra/develop/docker-compose.yml).

## Tự kiểm tra kế hoạch

Task 1 đáp ứng môi trường và bài Hasura; Task 2 giữ ranh giới nghiệp vụ và quyền; Task 3 đáp ứng retry, outbox và replay; Task 4 bao phủ hướng dẫn và kiểm chứng xuyên suốt. Năm Review Focus đều có test ở Task 1–4. Các patch version và cơ chế Kestra cần xác nhận bằng image thật được giữ thành bước kiểm chứng cụ thể trước khi viết workflow.

## Kết quả triển khai

Hoàn tất ngày 2026-11-04. Các điều chỉnh theo API Kestra thực tế, quyền trigger Hasura và khóa advisory cho outbox được ghi trong `demo/VERIFICATION.md` và ledger. Checkout không có Git nên không commit.
