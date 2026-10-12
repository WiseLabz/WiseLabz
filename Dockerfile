# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1/5: frontend — build the Vite/React/TS SPA -> web/dist/
# =============================================================================
# The builder stages run on the build host's platform ($BUILDPLATFORM) and
# cross-compile for the target, so multi-arch builds need no QEMU emulation.
# The SPA output is architecture-independent, so it is built once and shared.
FROM --platform=$BUILDPLATFORM oven/bun:1-alpine@sha256:d888c0ae6c86d7866ff10c5aafdd9077b36aee6455b33dd270fb93c0dd5cef6f AS frontend
WORKDIR /repo/web

# Install deps first, isolated from source changes, so `bun install` is cached.
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

# `bun run build` runs orval codegen via the npm "prebuild" lifecycle hook
# before tsc/vite, and orval.config.ts points at ../docs/openapi.yaml — so
# the spec must be present at that exact relative path before building.
COPY web/ ./
COPY docs/openapi.yaml /repo/docs/openapi.yaml
RUN bun run build

# =============================================================================
# Stage 2/5: gomod — Go toolchain with the module cache filled
# =============================================================================
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:738d1cf061836894ff6bb8c33881080ac66de8cf0586615012a0c8f592649cfa AS gomod

# git is required for `go build`'s automatic VCS stamping (debug.ReadBuildInfo,
# included in the operator diagnostics bundle) — the alpine golang image ships without it.
# ca-certificates is required for TLS to the Go module proxy during download.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy module files and resolve deps first, so this layer is cached unless
# go.mod/go.sum change.
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

# =============================================================================
# Stage 3/5: godeps — list the packages the server binary imports from outside
# its own module
# =============================================================================
# The list only changes when an import is added or removed, so the backend
# stage can compile those packages in a layer keyed on the list, not on the
# source. This stage re-runs on every source change; it is cheap (no compile).
# GOOS/GOARCH/CGO_ENABLED must match the real build so build constraints select
# the same files.
FROM gomod AS godeps
ARG TARGETOS TARGETARCH
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go list -deps -f '{{.ImportPath}} {{with .Module}}{{.Path}}{{end}}' ./cmd/server \
    | awk -v self="$(go list -m)" '$2 != self { print $1 }' | sort >/deps.txt

# =============================================================================
# Stage 4/5: backend — build the static Go binary
# =============================================================================
FROM gomod AS backend
ARG TARGETOS TARGETARCH

WORKDIR /src/backend

# Compile the dependencies (standard library included) before .git and the
# sources are copied. COPY --from is keyed on the content of /deps.txt, so
# while the import list is unchanged this layer is a cache hit even though
# every commit changes .git and the sources, and the final build below only
# compiles this module's own packages. The environment and -trimpath must
# match the final build or Go's build cache will not reuse the results.
COPY --from=godeps /deps.txt /deps.txt
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH xargs go build -trimpath </deps.txt

# The Go module lives in backend/, not the repo root, but VCS stamping only
# activates when the main package, its module, and the working directory all
# resolve to the same git repository. So .git must be copied as a sibling of
# backend/ here (mirroring the real repo layout), not just the backend
# subtree on its own.
COPY .git /src/.git
COPY backend/ /src/backend/

# Overlay the real frontend build over the placeholder committed at
# backend/internal/web/dist/ (see backend/internal/web/embed.go) so the
# `//go:embed all:dist` picks up the actual SPA instead of the placeholder.
COPY --from=frontend /repo/web/dist/. /src/backend/internal/web/dist/

# CGO_ENABLED=0 is safe: the backend is 100% pure-Go (modernc.org/sqlite for
# SQLite, jackc/pgx/v5/stdlib for Postgres) — no cgo/libc dependency, so the
# resulting binary is statically linked and runs on the cgo-less distroless
# base in stage 5.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/wiselabz ./cmd/server

# Pre-create the default SQLite data directory owned by the distroless
# "nonroot" UID/GID (65532:65532). distroless/static has no shell to mkdir
# or chown, so this has to happen in the (shell-having) builder stage and be
# copied in; Docker also uses an existing image-layer directory's ownership
# as the template when a volume is later mounted at the same path, which is
# what makes `docker run -v data:/data` work out of the box for the default
# file:/data/wiselabz.db DSN.
RUN mkdir -p /data && chown -R 65532:65532 /data

# =============================================================================
# Stage 5/5: final — minimal distroless runtime image
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS final

COPY --from=backend /out/wiselabz /wiselabz
COPY --from=backend --chown=65532:65532 /data /data

USER nonroot:nonroot

EXPOSE 8080
VOLUME ["/data"]

# Serve the embedded SPA build (server.embed); override at runtime if needed.
ENV WISELABZ_SERVER_EMBED=true

# distroless/static has no shell (no curl/wget), so HEALTHCHECK relies on the
# binary's own --healthcheck mode: it GETs its own /readyz and exits 0 only
# when the server is ready.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/wiselabz", "--healthcheck"]

ENTRYPOINT ["/wiselabz"]
