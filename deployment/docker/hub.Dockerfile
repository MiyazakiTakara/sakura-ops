# SakuraOps hub image (dev and CI). Build context is the repository root.
# Base images are pinned by digest; the tag is kept as a comment for humans.

# --- Stage 1: SvelteKit static build (Node 22, matches frontend engines) ---
# node:22-trixie-slim
FROM node@sha256:f0960f8dc047f358e3fb428f36ccba21732194fa5f0cac849f401e63e62fb9e6 AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# --- Stage 2: Go server with the embedded UI ---
# golang:1.27.4-trixie
FROM golang@sha256:430985542100b871e108ccc00e8dcc5eef08ca14ee5825e98ebf563d8ffec4ee AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY db/ ./db/
COPY --from=web /src/frontend/build/ ./internal/webui/dist/
RUN go build -trimpath -ldflags='-s -w' -o /out/sakuraops-server ./cmd/sakuraops-server \
 && go build -trimpath -ldflags='-s -w' -o /out/sakuraops-migrate ./cmd/sakuraops-migrate

# --- Stage 3a: one-shot migrator (target "migrator"): only sakuraops-migrate, no hub, no UI ---
# gcr.io/distroless/static-debian13:nonroot
FROM gcr.io/distroless/static-debian13@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS migrator
COPY --from=build /out/sakuraops-migrate /sakuraops-migrate
USER 65532:65532
ENTRYPOINT ["/sakuraops-migrate"]
CMD ["up"]

# --- Stage 3b: minimal non-root hub runtime, no shell (last stage = default target) ---
FROM gcr.io/distroless/static-debian13@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS hub
COPY --from=build /out/sakuraops-server /sakuraops-server
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/sakuraops-server"]
CMD ["-listen", "0.0.0.0:8080"]
