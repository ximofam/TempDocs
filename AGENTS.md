# Repository Guidelines

## Project Structure & Content

This repository currently contains the DX-OS Open-Core proposal, `DX-OS_Open-Core_OLP_2026.md`, and its architecture diagram, `DX-OS_Architecture.png`. The Markdown document is the project brief; it describes the architecture, requirements, and example technology stack. Keep diagrams and other referenced assets in the repository root unless the project grows enough to warrant dedicated folders. Preserve the image link in the brief when renaming or moving the diagram.

## Editing the Proposal

The proposal is written in Vietnamese. Keep new material in Vietnamese and retain established technical terms and abbreviations (for example, H-P-D-I, SSO, API, and GitOps). Use Markdown headings to maintain a clear hierarchy, concise paragraphs for explanations, and bullets for enumerated capabilities. When changing architecture requirements, update related sections consistently and check that claims match the diagram and linked assets. Avoid adding implementation details that are not part of the proposal.

## Build, Test, and Review

There is no application source code or configured build, test, or lint command in the current repository. For document-only changes, review the rendered Markdown and verify links and image references manually. Useful checks include:

- `rg '^#{1,6} ' DX-OS_Open-Core_OLP_2026.md` to inspect heading structure.
- `test -f DX-OS_Architecture.png` to confirm the referenced diagram is present.

If code or automated checks are introduced, document the relevant setup and commands here.

## Style and Naming

Use UTF-8, standard Markdown, descriptive headings, and consistent punctuation. Keep filenames readable and use hyphens or underscores consistently with the existing proposal filename. Prefer plain, direct wording and explain specialized abbreviations on first use when practical.

## Commits and Pull Requests

Git history is not available in this checkout, so no established commit convention can be confirmed. Write concise commit subjects that state the change, such as `docs: clarify core platform requirements`. Pull requests should summarize the proposal sections affected, explain material architecture changes, and include rendered screenshots only when they help reviewers assess a visual change. Check all changed links and assets before requesting review.

## Demo Hasura và Kestra

`demo/` chứa Go API/CLI, PostgreSQL, Hasura và Kestra để học tích hợp theo sơ đồ cải tiến. Hướng dẫn tiếng Việt bắt đầu ở `demo/README.md`.

- Setup: `cd demo`, `cp .env.example .env`, `make up`, `make bootstrap`.
- Kiểm tra cấu hình: `docker compose config --quiet` trong `demo/`.
- Kiểm thử đầy đủ với PostgreSQL thật: `make test`; xuyên suốt Hasura/Kestra/Go: `make smoke`.
- Nếu có Go 1.26+: `go test ./...`, `go vet ./...`, `go build -buildvcs=false ./...` trong `demo/`; test DB sẽ skip nếu thiếu `TEST_DATABASE_URL`.
- Dừng giữ dữ liệu: `make down`. `make reset` hỏi xác nhận trước khi xóa volume dữ liệu demo.
- Giữ mã/config/hướng dẫn demo trong `demo/`, dùng SPDX Apache-2.0 cho mã nguồn tự viết, không sửa tài liệu kiến trúc gốc để khớp các giản lược của bài học.
