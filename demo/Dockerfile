# SPDX-License-Identifier: Apache-2.0
FROM golang:1.26.1-alpine AS test
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["go", "test", "./...", "-count=1"]
FROM test AS build
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/demo ./cmd/demo
FROM alpine:3.23.3
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ /app/
USER 65532:65532
ENTRYPOINT ["/app/demo"]
