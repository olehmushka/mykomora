# syntax=docker/dockerfile:1

# core-api image. Three stages share one dependency layer:
#   dev      -- live reload, source bind-mounted by Compose
#   builder  -- static binaries (api, migrate)
#   migrate  -- one-shot migration runner for production
#   runtime  -- distroless, non-root (the default target)

FROM golang:1.26-alpine AS base
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# --- development -----------------------------------------------------------
FROM base AS dev
# Warm the tool cache so the first reload is not a cold build. `air` is pinned
# as a tool directive in go.mod, so no version floats here.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /dev/null github.com/air-verse/air
COPY . .
EXPOSE 8081
CMD ["go", "tool", "air", "-c", ".air.toml"]

# --- build -----------------------------------------------------------------
FROM base AS builder
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- migrate ---------------------------------------------------------------
# One-shot production migration runner. cmd/migrate reads the .sql files from
# db/migrations relative to the working directory, so they ship beside it.
FROM gcr.io/distroless/static-debian12:nonroot AS migrate
WORKDIR /
COPY --from=builder /out/migrate /migrate
COPY --from=builder /app/db/migrations /db/migrations
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]
CMD ["up"]

# --- runtime ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=builder /out/api /api
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/api"]
