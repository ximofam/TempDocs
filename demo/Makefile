# SPDX-License-Identifier: Apache-2.0
.PHONY: up bootstrap test smoke down reset
up:
	docker compose up -d --build postgres hasura kestra api
bootstrap:
	docker compose run --rm demo bootstrap
test:
	docker compose run --rm test
smoke:
	docker compose run --rm demo smoke
down:
	docker compose down
reset:
	@printf 'Xóa toàn bộ dữ liệu demo dxos-learning? Gõ DELETE: '; read answer; test "$$answer" = DELETE
	docker compose down -v
