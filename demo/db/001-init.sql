-- SPDX-License-Identifier: Apache-2.0
-- Chỉ được entrypoint PostgreSQL chạy khi volume còn trống.
CREATE ROLE kestra LOGIN PASSWORD 'demo-kestra';
CREATE DATABASE kestra OWNER kestra;
CREATE ROLE hasura_meta LOGIN PASSWORD 'demo-meta';
CREATE DATABASE hasura_metadata OWNER hasura_meta;
CREATE ROLE hasura_runtime LOGIN PASSWORD 'demo-hasura';
CREATE ROLE api_runtime LOGIN PASSWORD 'demo-api';
CREATE SCHEMA core;
CREATE SCHEMA procurement;
CREATE TABLE core.work_items (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id text NOT NULL,
 title text NOT NULL CHECK (length(trim(title)) > 0), created_by text NOT NULL,
 UNIQUE (workspace_id,id)
);
CREATE TABLE procurement.requests (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id text NOT NULL,
 work_item_id uuid NOT NULL, created_by text NOT NULL,
 description text NOT NULL DEFAULT '', amount numeric(14,2) NOT NULL CHECK(amount>0),
 status text NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','PENDING_APPROVAL','APPROVED','REJECTED')),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 UNIQUE(workspace_id,id),
 FOREIGN KEY(workspace_id,work_item_id) REFERENCES core.work_items(workspace_id,id)
);
CREATE FUNCTION procurement.guard_draft() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.description,NEW.amount,NEW.work_item_id,NEW.created_by,NEW.workspace_id,NEW.id)
    IS DISTINCT FROM (OLD.description,OLD.amount,OLD.work_item_id,OLD.created_by,OLD.workspace_id,OLD.id) THEN
  IF OLD.status <> 'DRAFT' OR NEW.status <> 'DRAFT' THEN RAISE EXCEPTION 'only draft content can change'; END IF;
  IF (NEW.created_by,NEW.workspace_id,NEW.id) IS DISTINCT FROM (OLD.created_by,OLD.workspace_id,OLD.id) THEN RAISE EXCEPTION 'identity is immutable'; END IF;
  NEW.version := OLD.version+1;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_draft BEFORE UPDATE ON procurement.requests FOR EACH ROW EXECUTE FUNCTION procurement.guard_draft();
CREATE TABLE core.outbox (
 event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 event_type text NOT NULL CHECK(event_type='request.submitted'), workspace_id text NOT NULL,
 request_id uuid NOT NULL, version integer NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,request_id) REFERENCES procurement.requests(workspace_id,id),
 UNIQUE(request_id,version,event_type)
);
CREATE TABLE procurement.approval_inbox (
 event_id uuid PRIMARY KEY REFERENCES core.outbox(event_id), workspace_id text NOT NULL,
 request_id uuid NOT NULL, version integer NOT NULL,
 FOREIGN KEY(workspace_id,request_id) REFERENCES procurement.requests(workspace_id,id),
 UNIQUE(request_id,version)
);
CREATE TABLE core.processed_events (
 event_id uuid PRIMARY KEY REFERENCES core.outbox(event_id), result text NOT NULL,
 processed_at timestamptz NOT NULL DEFAULT now()
);
GRANT USAGE ON SCHEMA core,procurement TO hasura_runtime,api_runtime;
GRANT SELECT,INSERT,UPDATE ON core.work_items,procurement.requests TO hasura_runtime;
GRANT SELECT ON procurement.approval_inbox,core.outbox TO hasura_runtime;
-- Hasura cần tạo SQL trigger/schema hdb_catalog cho Event Triggers; không phải superuser.
GRANT CREATE ON DATABASE dxos TO hasura_runtime;
GRANT TRIGGER ON core.outbox TO hasura_runtime;
GRANT SELECT ON core.work_items,procurement.requests,core.outbox,procurement.approval_inbox,core.processed_events TO api_runtime;
GRANT UPDATE(status,version) ON procurement.requests TO api_runtime;
GRANT INSERT ON core.outbox,procurement.approval_inbox,core.processed_events TO api_runtime;
INSERT INTO core.work_items(id,workspace_id,title,created_by) VALUES
 ('10000000-0000-0000-0000-000000000001','ws-a','Bảo trì máy bơm A','alice'),
 ('10000000-0000-0000-0000-000000000002','ws-b','Bảo trì máy bơm B','bob') ON CONFLICT DO NOTHING;
INSERT INTO procurement.requests(id,workspace_id,work_item_id,created_by,description,amount) VALUES
 ('20000000-0000-0000-0000-000000000001','ws-a','10000000-0000-0000-0000-000000000001','alice','Mua vòng bi',500000),
 ('20000000-0000-0000-0000-000000000002','ws-b','10000000-0000-0000-0000-000000000002','bob','Mua dầu máy',300000) ON CONFLICT DO NOTHING;
