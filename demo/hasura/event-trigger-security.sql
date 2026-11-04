-- SPDX-License-Identifier: Apache-2.0
-- Trigger Hasura chạy với quyền người INSERT outbox. Cấp đúng quyền
-- ghi các cột event_log mà hàm insert_event_log dùng; không cấp quyền
-- đọc/sửa/xóa catalog hay quản trị metadata cho API.
-- GRANT được giữ khi Hasura tạo lại function trong lần reload metadata.
GRANT USAGE ON SCHEMA hdb_catalog TO api_runtime;
GRANT INSERT (id, schema_name, table_name, trigger_name, payload)
  ON hdb_catalog.event_log TO api_runtime;
