# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS build
WORKDIR /workspace
COPY --from=kernel . ./kernel-go
COPY --from=protocol . ./protocol
COPY --from=platform-protocol . ./platform-protocol
COPY . ./backend
WORKDIR /workspace/backend
RUN go mod edit \
	-replace=github.com/lvtuopen-ai/kernel-go=/workspace/kernel-go \
	-replace=github.com/lvtuopen-ai/liveshop-identity/protocol=/workspace/protocol && \
	go mod edit -replace=github.com/liveshop-platform/contracts=/workspace/platform-protocol && \
	CGO_ENABLED=0 go build -trimpath -o /out/identity ./internal/identity/cmd
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity-migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity-authorization-import ./cmd/authorizationimport
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity-permission-admin ./cmd/permissionadmin

FROM alpine:3.21
RUN addgroup -S liveshop && adduser -S -G liveshop liveshop
COPY --from=build /out/identity /app/identity
COPY --from=build /out/identity-migrate /app/identity-migrate
COPY --from=build /out/identity-authorization-import /app/identity-authorization-import
COPY --from=build /out/identity-permission-admin /app/identity-permission-admin
COPY --from=build /workspace/backend/configs /app/configs
COPY --from=build /workspace/backend/migrations /app/migrations
WORKDIR /app
USER liveshop:liveshop
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD nc -z 127.0.0.1 8092 || exit 1
ENTRYPOINT ["/app/identity"]
