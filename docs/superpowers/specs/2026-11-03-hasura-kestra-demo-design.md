# Thiết kế bộ demo Hasura và Kestra bằng Go

## Mục đích và lựa chọn đã chốt

Người học chưa biết cách dùng Hasura và Kestra, muốn chương trình thực hành trong `./demo` để hiểu cách tích hợp vào DX-OS. Người dùng chọn Go và bộ bài học tăng dần dùng chung Docker Compose.

Thiết kế bám theo `DX-OS_So-do-He-thong.md`: công việc chung nằm trong `core.work_items`, hồ sơ mua sắm có bảng và trạng thái riêng; CRUD qua Hasura, lệnh nghiệp vụ qua API Go, Kestra điều phối và thử lại. Không dùng mô hình `shared.tasks` trong kế hoạch cũ. Không sửa các tài liệu kiến trúc gốc trong phạm vi này.

## Các hướng đã cân nhắc

1. **Bài học tăng dần, chung Docker Compose — được chọn:** tách kiến thức thành ba bài nhưng tái sử dụng dịch vụ và dữ liệu; dễ quan sát từng thành phần, đổi lại cần chạy nhiều container.
2. **Ứng dụng mua sắm hoàn chỉnh ngay từ đầu:** thấy kết quả tích hợp sớm nhưng cấu hình và quy tắc nghiệp vụ dồn cùng lúc, khó chẩn đoán cho người mới.

## Mục tiêu và giới hạn

- Chạy các dịch vụ thật: PostgreSQL, Hasura GraphQL Engine v2, Kestra và API Go.
- Mỗi bài có mục tiêu, giải thích vai trò, lệnh chạy, request mẫu, kết quả mong đợi và cách xem lỗi trong giao diện dịch vụ.
- Chứng minh cô lập workspace ở Hasura, kiểm quyền tại API lệnh, kiểm tra phiên bản và chống tác động trùng khi workflow được chạy lại.
- Không triển khai toàn bộ DX-OS, Kubernetes, Operator, Keycloak, Gateway, AI hoặc giao diện nghiệp vụ riêng. Dùng Hasura Console và Kestra UI để quan sát.
- Không dạy cách để workflow tự phê duyệt thay con người hoặc cho CRUD sửa trực tiếp trạng thái quan trọng.

## Cấu trúc và cách chạy

Mọi mã nguồn, cấu hình, dữ liệu mẫu và hướng dẫn demo nằm trong `demo/`. Một Go module cung cấp API và CLI thực hành, với mã theo từng trách nhiệm rõ ràng. README tiếng Việt là điểm bắt đầu; ba bài có hướng dẫn riêng, liên kết tới file mã/config cần đọc.

Docker Compose cung cấp PostgreSQL, Hasura, Kestra và API Go. PostgreSQL có database/tài khoản riêng cho dữ liệu nghiệp vụ và kho trạng thái Kestra. Hasura có tài khoản runtime riêng với tài khoản khởi tạo schema. Bootstrap nạp schema, dữ liệu mẫu, Hasura metadata và Kestra flows; chạy lại không xóa dữ liệu hoặc tạo cấu hình trùng. Có lệnh reset riêng, ghi rõ sẽ xóa dữ liệu demo.

Mặc định chỉ công bố các cổng cần thiết trên localhost. Có `.env.example`, healthcheck và hướng dẫn xử lý cổng trùng. Chốt phiên bản image/API theo tài liệu chính thức khi viết kế hoạch; không dùng tag `latest`. Không cần dịch vụ trả phí hay tài khoản cloud. Tất cả file mã nguồn tự viết có SPDX Apache-2.0, dùng UTF-8 và LF.

Các lựa chọn do người triển khai đề xuất: dùng Go HTTP server và CLI, driver PostgreSQL thông dụng; Docker Compose là môi trường chính. Không cần cài Go trên máy nếu dùng container. Có thể chạy Go trực tiếp để đọc và debug. Hướng dẫn nêu yêu cầu tài nguyên thực tế sau khi kiểm thử.

## Bài 1 — Hasura CRUD và phân quyền

Schema tối thiểu gồm `core.work_items` và `procurement.requests`, liên kết bằng định danh, workspace được giữ nhất quán. Hồ sơ có tiêu đề/nội dung, số tiền dương, người tạo, trạng thái và phiên bản. Seed hai workspace, hai người tạo và một người duyệt cho mỗi workspace.

Học track bảng, quan hệ, query, mutation và subscription. Role nhân viên đọc dữ liệu đúng workspace, tạo/sửa bản nháp của mình và không sửa trạng thái, phiên bản, người tạo hay workspace qua GraphQL. Preset lấy workspace và người tạo từ session. Sửa nội dung bản nháp tăng phiên bản bằng cơ chế DB để mọi đường sửa tuân cùng quy tắc; dữ liệu đang gửi duyệt bị khóa sửa.

Để tránh đưa thêm SSO vào bài đầu, CLI Go phát JWT demo ký bằng khóa local, chứa claim role, người dùng và workspace; Hasura xác minh JWT. API Go xác minh chữ ký, hạn token và cùng danh tính. Đây là cơ chế học local, không phải triển khai SSO. Khóa và danh tính demo được giải thích rõ; không dùng admin secret cho request học phân quyền. Bootstrap giữ admin secret riêng.

Bài học có request hợp lệ, đọc workspace khác bị lọc và mutation sửa trường ngoài quyền bị từ chối. Giải thích role Hasura không tự trở thành role PostgreSQL. Quyền DB của từng dịch vụ là cơ chế riêng.

## Bài 2 — Kestra gọi Go và thử lại

Một flow chạy thủ công với input, gọi endpoint thực hành trên Go và ghi kết quả để xem execution/task logs. Endpoint lỗi tạm thời được bật rõ ràng cho bài học, có cách reset để tái hiện retry; không gây lỗi ngẫu nhiên. Workflow có số lần thử và thời gian chờ hữu hạn, không có side effect nghiệp vụ trong bài này.

Học namespace, flow, task, input, HTTP request, timeout và retry. Đưa cả tình huống thành công sau retry và lỗi kéo dài tới trạng thái failed vào hướng dẫn. Không cần Python trong flow; ưu tiên HTTP task có sẵn.

## Bài 3 — Tích hợp Hasura → Kestra → Go

Luồng nghiệp vụ hẹp: nhân viên tạo bản nháp qua Hasura, gửi lệnh `submit` tới Go; Go kiểm danh tính, quyền sở hữu, workspace, trạng thái, số tiền và expected version, rồi giao dịch chuyển `DRAFT` sang `PENDING_APPROVAL`, tăng phiên bản và tạo sự kiện outbox.

Hasura Event Trigger trên bảng outbox gọi webhook Kestra. Payload có event ID nghiệp vụ, workspace, request ID và phiên bản gửi duyệt. Kestra gọi API nội bộ Go để tạo mục hộp duyệt cho phiên bản này. Người duyệt dùng CLI gọi lệnh `approve` hoặc `reject` trên đúng phiên bản; Kestra không quyết định thay người duyệt. Có thể quan sát trạng thái bằng query/subscription Hasura.

API nội bộ dùng credential dịch vụ riêng, không dùng JWT nhân viên hoặc Hasura admin secret. Mỗi request được kiểm tra workspace và hồ sơ trong DB; dữ liệu payload không tự cấp quyền. Credential webhook và dịch vụ chỉ tồn tại ở cấu hình backend; cấu hình theo cơ chế được bản Kestra chọn hỗ trợ và được giải thích trong README.

Tạo mục hộp duyệt dùng unique constraint theo event ID, kiểm tra loại sự kiện, trạng thái và phiên bản; ghi nhận sự kiện đã xử lý cùng giao dịch với tác động. Nếu callback bị gửi lại sau khi đã xử lý, trả kết quả thành công cũ và không thêm tác động. Sự kiện chưa xử lý mà phiên bản đã cũ được ghi nhận là bỏ qua có lý do; không áp dụng lên hồ sơ mới. Phê duyệt kiểm tra role người duyệt, workspace, không tự duyệt hồ sơ của mình, expected version và trạng thái, rồi cập nhật nguyên tử.

Giải thích rõ HTTP webhook được nhận chỉ có nghĩa execution được tạo, không bảo đảm workflow hoàn tất. Hasura thử giao webhook lại khi không nhận thành công; Kestra retry task khi callback lỗi. Nếu execution thất bại hẳn, người học xem lỗi và chạy lại/replay bằng dữ liệu mẫu; demo không tuyên bố có bộ phục hồi workflow tự động đầy đủ.

CLI có lệnh/tình huống tái gửi event để chứng minh chỉ một mục hộp duyệt; gửi expected version cũ và dùng tài khoản sai quyền để thấy bị từ chối. Không đưa migration audit hash-chain hoặc workflow dài nhiều ứng dụng vào bài này.

## Kiểm chứng và tiêu chí hoàn thành

- Go unit/integration tests tập trung vào quyền hành động, phiên bản và giao dịch chống trùng; không chỉ test cấu trúc YAML.
- Kiểm tra Compose, bootstrap hai lần và chạy smoke test xuyên suốt trên dịch vụ thật khi môi trường cho phép.
- Kiểm chứng JWT hợp lệ/hết hạn/sai chữ ký; nhân viên không đọc workspace khác, không sửa trạng thái qua Hasura; người ngoài quyền không duyệt được.
- Workflow thực hiện HTTP task và retry có thể quan sát; sự kiện được gửi từ Hasura thật tới Kestra và callback tới Go.
- Hai lần callback cùng sự kiện, kể cả đồng thời, chỉ tạo một mục hộp duyệt; phê duyệt phiên bản cũ thất bại.
- README nêu URL giao diện, thứ tự học, lệnh kiểm thử, dừng/reset và phạm vi giản lược. Cập nhật AGENTS.md về setup/check mới nếu thêm mã.
- Báo rõ phần nào đã chạy thật và phần nào chưa kiểm chứng do giới hạn môi trường.

## Quyết định để người triển khai chốt

Tên thư mục con, endpoint/CLI cụ thể, driver Go, phiên bản phụ thuộc, chi tiết bootstrap và các status code được tự chọn nhất quán trong kế hoạch. Nếu cơ chế webhook của phiên bản Kestra cần adapter thì nêu rõ trong kế hoạch và giữ adapter tối thiểu, không giả lập Kestra. Repository hiện không có Git nên không thể commit thiết kế trong checkout này.
