# DX-OS Open-Core — Đề xuất cải thiện kiến trúc và kế hoạch triển khai

Tài liệu này đề xuất các cải tiến dựa trên việc đối chiếu [đề bài DX-OS Open-Core OLP 2026](DX-OS_Open-Core_OLP_2026.md), [tổng quan kiến trúc](architecture_overview.md) và [kế hoạch triển khai](implementation_plan.md).

**Ưu tiên sửa mô hình dữ liệu, phân quyền và độ tin cậy của workflow trước; sau đó hoàn thiện Operator và các điểm nhấn trình diễn.** Mục tiêu là có một luồng nghiệp vụ chạy xuyên suốt, chứng minh được yêu cầu đề thi và chịu được các tình huống lỗi.

Các lựa chọn dưới đây là đề xuất để đội thi xem xét, chưa phải các quyết định kiến trúc đã được chốt.

Xem [sơ đồ kiến trúc hệ thống và luồng nghiệp vụ](DX-OS_So-do-He-thong.md), gồm ảnh PNG, SVG và bản Mermaid có thể chỉnh sửa.

## 1. Chọn một kiến trúc thống nhất cho cả hai tài liệu

Hai lựa chọn đều khả thi, nhưng có chi phí khác nhau:

| Hướng | Lợi ích | Phần phải làm thêm |
|---|---|---|
| **Giữ Hasura — phương án khuyến nghị** | Tận dụng GraphQL, subscription, phân quyền và Event Triggers đã có trong kế hoạch | Làm rõ quyền ở Hasura, tài khoản DB và xác thực các dịch vụ nền |
| Chuyển sang PostgREST | Phù hợp với mục tiêu thực thi quyền bằng PostgreSQL role/RLS | Bổ sung cơ chế sự kiện bền vững và cập nhật thời gian thực |

Hasura áp dụng quyền khi biên dịch truy vấn; PostgREST chuyển sang PostgreSQL role tương ứng với JWT. Đây là hai mô hình thực thi khác nhau. Tham khảo [phân quyền Hasura](https://hasura.io/docs/2.0/auth/authorization/permissions/) và [cơ chế role của PostgREST](https://docs.postgrest.org/en/stable/references/auth.html).

### Việc cần làm

- Ghi một quyết định kiến trúc ngắn, chọn một hướng thực thi.
- Cập nhật đồng bộ sơ đồ, manifest, API, workflow và test trong hai tài liệu.
- Mô tả rõ nơi thực thi quyền, tài khoản kết nối DB và cách truyền danh tính tới các dịch vụ.
- Nếu giữ Hasura, bỏ giả định rằng role Hasura cùng tên sẽ tự động trở thành role PostgreSQL.

Với kế hoạch hiện tại, giữ Hasura giúp giảm phần phải thiết kế lại.

## 2. Giữ dữ liệu công việc chung, tách hồ sơ nghiệp vụ riêng

Đề xuất thay bảng đa hình chứa mọi thuộc tính bằng mô hình:

| Dữ liệu | Trách nhiệm |
|---|---|
| `core.work_items` | Định danh công việc, workspace, tiêu đề, người phụ trách, hạn xử lý |
| `procurement.requests` | Hồ sơ mua sắm và trạng thái phê duyệt |
| `procurement.request_items` | Các dòng hàng trong yêu cầu |
| `timesheet.entries` | Giờ công theo người, ngày và công việc |
| `maintenance.orders` | Hồ sơ bảo trì và trạng thái xử lý riêng |

Các bảng nghiệp vụ liên kết với công việc chung bằng định danh. Trạng thái mua sắm và giờ công được quản lý độc lập.

**Giá trị chia sẻ dữ liệu vẫn giữ nguyên:** các ứng dụng đọc cùng thông tin công việc gốc. Chấp nhận `JOIN` khi cần biểu diễn đúng quan hệ nghiệp vụ.

### Việc cần làm

- Phân biệt thông tin chung với dữ liệu thuộc từng nghiệp vụ.
- Xác định quan hệ một–nhiều, vòng đời và trạng thái của từng loại hồ sơ.
- Khai báo rõ ai sở hữu dữ liệu, ứng dụng nào được đọc và ứng dụng nào được sửa.
- Tách việc kiểm tra tương thích schema khỏi việc cấp quyền chia sẻ.
- Không tự cấp quyền chỉ vì ứng dụng mới đăng ký một cột trùng tên và kiểu.

## 3. Xây dựng một mô hình phân quyền xuyên suốt

Cần phân biệt ba chiều:

- **Workspace:** người dùng thuộc đơn vị nào.
- **Ứng dụng:** được sử dụng mua sắm, giờ công hay bảo trì.
- **Hành động:** được tạo, gửi duyệt, phê duyệt hay quản trị.

Ví dụ, quyền dùng ứng dụng mua sắm không mặc nhiên bao gồm quyền duyệt.

`workspace_id` và `created_by` phải được xác định hoặc kiểm tra từ danh tính đã xác thực. Hasura hỗ trợ column presets để lấy giá trị từ session và ngăn người dùng tự ghi đè trường đó. Tham khảo [Column Presets](https://hasura.io/docs/2.0/auth/authorization/permissions/column-presets/).

### Việc cần làm

- Lập ma trận quyền theo workspace, ứng dụng và hành động.
- Áp dụng cùng phạm vi quyền cho CRUD, AI, workflow, tệp và báo cáo.
- Đối chiếu namespace và workspace trong yêu cầu RAG với quyền của người gọi.
- Tách tài khoản chạy ứng dụng khỏi tài khoản migration/Operator.
- Không dùng superuser cho các truy vấn thường ngày.
- Kiểm chứng cơ chế quyền bằng request thực qua các đường truy cập được hỗ trợ.

**Tiêu chí nghiệm thu:** đổi `workspace_id`, namespace hoặc ID hồ sơ trong request vẫn không đọc hay sửa được dữ liệu ngoài quyền.

## 4. Tách CRUD khỏi các hành động nghiệp vụ quan trọng

Cho phép Auto-API xử lý việc đọc và chỉnh sửa thông tin thông thường. Các hành động `submit`, `approve`, `reject`, `cancel` cần có đường xử lý riêng.

Mỗi hành động phải kiểm tra người thực hiện, trạng thái hiện tại và phiên bản hồ sơ. Khi phê duyệt, cần lưu rõ phiên bản dữ liệu được duyệt; sửa nội dung quan trọng sau đó phải tạo một vòng duyệt mới.

### Phân chia trách nhiệm

- **Database:** thực thi quy tắc đơn giản như số tiền dương, trường bắt buộc và khóa ngoại.
- **Phần xử lý lệnh nghiệp vụ:** kiểm tra quyền hành động, chuyển trạng thái và các quy tắc tổng hợp trong giao dịch phù hợp.
- **Kestra:** điều phối tác vụ, chờ kết quả, gọi OCR và gửi thông báo.

Các quy tắc như tổng giờ trong ngày hoặc hạn mức ngân sách cần xét nhiều bản ghi. Quyết định nghiệp vụ phải được kiểm tra rõ ràng trước khi được ghi nhận.

### Việc cần làm

- Định nghĩa trạng thái và các phép chuyển hợp lệ của từng loại hồ sơ.
- Xác định ai được duyệt và có được tự duyệt hay không.
- Kiểm soát phiên bản để kết quả duyệt cũ không áp dụng lên hồ sơ đã thay đổi.
- Hạn chế quyền cập nhật trực tiếp các trường trạng thái quan trọng qua CRUD.

## 5. Thiết kế workflow chịu được gửi trùng, sai thứ tự và khởi động lại

Nếu giữ Hasura, tận dụng Event Triggers và bổ sung:

- Định danh sự kiện và phiên bản hồ sơ.
- Cơ chế nhận diện sự kiện đã xử lý.
- Quy tắc từ chối kết quả của phiên bản cũ.
- Chính sách thử lại và danh sách công việc lỗi cần xử lý.
- Cơ chế tránh tạo tác động trùng khi một bước được chạy lại.

Hasura có thể giao sự kiện nhiều lần và sai thứ tự, nên các quy tắc này cần được thiết kế ngay từ đầu. Tham khảo [Hasura Event Triggers](https://hasura.io/docs/2.0/event-triggers/create-trigger/).

**Tiêu chí nghiệm thu:** dừng Kestra giữa chừng, khởi động lại và gửi lại cùng sự kiện vẫn chỉ tạo một kết quả nghiệp vụ hợp lệ.

## 6. Thu gọn Operator thành bộ điều phối cấu hình có trạng thái rõ ràng

Operator tập trung vào đăng ký ứng dụng, cấu hình quyền, API, workflow và kiểm tra sai lệch.

Mỗi lần triển khai cần thể hiện trạng thái như `Pending`, `Applying`, `Ready`, `Failed`, kèm phiên bản manifest và bước đang lỗi. Ứng dụng chỉ được công bố sẵn sàng khi các thành phần bắt buộc đã cấu hình thành công.

### Việc cần làm

- Xác định trạng thái hoàn thành từng bước và cách tiếp tục sau lỗi.
- Bảo đảm áp dụng lại không gây mất quyền ngoài ý muốn hoặc tạo cấu hình trùng.
- Phối hợp các thao tác schema đồng thời để metadata và quyền giữ được tính nhất quán.
- Trong môi trường GitOps, dùng Git làm nguồn cấu hình chuẩn.
- Phân định phần ArgoCD quản lý và phần Operator sinh ra để tránh ghi đè nhau.
- Đối soát cả quyền thiếu lẫn quyền thừa.
- Giới hạn các thay đổi schema được hỗ trợ trong phiên bản đầu; quy định riêng cho thay đổi kiểu, xóa dữ liệu và gỡ ứng dụng.

**Tiêu chí nghiệm thu:** làm một bước triển khai thất bại rồi chạy lại phải đưa hệ thống về trạng thái đúng; reconcile xử lý được cả quyền thiếu và quyền thừa.

## 7. Bổ sung các năng lực đề thi bằng một kịch bản liên thông

Đề xuất dùng **bảo trì thiết bị** làm kịch bản xuyên suốt:

> Cảm biến vượt ngưỡng → cập nhật trạng thái thiết bị → tạo công việc bảo trì → đề xuất mua vật tư → phê duyệt → ghi giờ công → đóng hồ sơ → đưa dữ liệu vào báo cáo lịch sử.

Kịch bản này giúp chứng minh các yêu cầu bằng hành vi thực:

| Năng lực | Phần cần triển khai |
|---|---|
| IT/OT | Bộ phát dữ liệu cảm biến mô phỏng, API tiếp nhận và trạng thái thiết bị |
| Dữ liệu phi cấu trúc | Upload → cách ly → quét mã độc → cho phép tải xuống/OCR |
| Nóng/lạnh | Xuất hồ sơ đã đóng theo phiên bản, tránh xuất trùng và tính trùng |
| Phả hệ | Từ chỉ số báo cáo truy ngược về hồ sơ, phiên bản và phép biến đổi |
| GitOps | Thay đổi cấu hình qua Git và chứng minh hệ thống hội tụ về trạng thái mong muốn |

### Củng cố audit

- Bổ sung cơ chế ghi đồng thời an toàn cho chuỗi băm.
- Bảo vệ checkpoint độc lập với quyền sửa dữ liệu nghiệp vụ.
- Kiểm chứng định kỳ và ghi nhận kết quả đối soát.
- Mô tả chính xác phạm vi phát hiện sửa đổi mà hệ thống thực sự bảo đảm.

## 8. Viết lại kế hoạch theo kết quả kiểm chứng được

Đề xuất sắp xếp lại thứ tự:

| Chặng | Kết quả cần đạt trước khi đi tiếp |
|---|---|
| **1. Chốt hợp đồng** | Một lựa chọn Hasura/PostgREST, mô hình dữ liệu, ma trận quyền và API thống nhất |
| **2. Luồng tối thiểu** | Đăng nhập → tạo hồ sơ → gửi duyệt → phê duyệt → xem audit |
| **3. Kiểm tra độ tin cậy** | Truy cập chéo tenant bị chặn; sự kiện trùng, dữ liệu cũ và khởi động lại được xử lý đúng |
| **4. Tự động hóa** | Manifest cấu hình được ứng dụng; Operator phục hồi được lỗi triển khai |
| **5. Đáp ứng đề thi** | IT/OT, kiểm dịch tệp, nóng/lạnh và phả hệ chạy trong kịch bản chung |
| **6. Đóng gói** | Cài từ gói phát hành ở thư mục mới và chạy lại toàn bộ kịch bản |

### Yêu cầu đối với mỗi task

- Chỉ rõ đầu vào đã tồn tại và các task phụ thuộc.
- Định nghĩa đầu ra và hợp đồng giữa các thành phần.
- Mô tả cách xử lý lỗi quan trọng.
- Có bài kiểm tra nghiệm thu phù hợp với cam kết của task.

### Các điểm cần sửa trong kế hoạch hiện tại

- Loại bỏ phần Go/Python lẫn nhau trong Operator và các bài test.
- Sửa thứ tự task để manifest, cấu hình và fixture có trước các bài kiểm tra sử dụng chúng.
- Đồng bộ hợp đồng request/response của AI, workflow và giao diện.
- Bổ sung kiểm thử xuyên suốt cho đăng nhập, tạo hồ sơ, phân quyền, phê duyệt và audit.
- Bổ sung tình huống truy cập trái quyền, gửi lặp và khởi động lại giữa chừng.
- Bảo đảm lệnh kiểm tra cuối chạy đủ Go, Python và frontend.
- Kiểm tra khả năng chạy của gói cài đặt, thay vì chỉ kiểm tra sự tồn tại của file.

Các tuyên bố như “30 giây”, “<20 MB” hay “giảm 80%” cần trở thành **mục tiêu đo lường**, có điều kiện đo cụ thể. Phần trình diễn ưu tiên bằng chứng: thêm ứng dụng, chặn truy cập trái quyền, phục hồi workflow và truy ngược số liệu báo cáo.
