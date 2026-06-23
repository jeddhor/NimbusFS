# syntax=docker/dockerfile:1

# --- frontend ---
FROM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- backend ---
# cgo is required: PAM auth links libpam, and go-sqlite3 compiles the
# sqlite C amalgamation directly into the binary (no libsqlite3 needed
# at runtime, but a C toolchain is needed here to build it).
FROM golang:1.25-bookworm AS backend
RUN apt-get update && apt-get install -y --no-install-recommends libpam0g-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/nimbusfs ./cmd/nimbusfs

# --- runtime ---
FROM debian:bookworm-slim

# shared-mime-info provides /usr/share/mime/packages/freedesktop.org.xml,
# the extension -> human-readable type name database (internal/mimetypes)
# used to label files in the UI (e.g. "Python script" instead of "PY
# File"). A few MB, no heavy transitive deps.
#
# ffmpeg/poppler-utils are only used opportunistically for video/PDF
# thumbnails (internal/thumbnail) and pull in a surprisingly large
# transitive dependency tree on Debian (SDL2, X11, Mesa — none of which a
# headless frame-grab actually needs), roughly 600MB vs ~120MB without
# them. Set --build-arg WITH_THUMBNAIL_TOOLS=false for a smaller image;
# video/PDF files will just show a generic icon instead of a thumbnail.
ARG WITH_THUMBNAIL_TOOLS=true
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpam0g \
    ca-certificates \
    shared-mime-info \
    $(if [ "$WITH_THUMBNAIL_TOOLS" = "true" ]; then echo ffmpeg poppler-utils; fi) \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend /out/nimbusfs /usr/local/bin/nimbusfs
COPY deploy/pam.d/nimbusfs /etc/pam.d/nimbusfs

# Runs as root by design: PAM authentication and per-request impersonation
# of the authenticated Linux user (so every filesystem syscall is checked
# against that user's real permissions, not root's) both require it — see
# internal/fsops/identity.go. Do not add a USER directive or drop
# CAP_SETUID/CAP_SETGID; auth.pam and per-user permission enforcement will
# silently stop working.
#
# auth.pam needs the container to share the host's user database and PAM
# stack — bind-mount /etc/passwd, /etc/group, /etc/shadow, and /etc/pam.d
# read-only (see README "Docker" section). If you'd rather not do that,
# use auth.proxy_auth or auth.ssh_keys instead, which only need the named
# accounts to resolve (e.g. via /etc/passwd) and don't need /etc/shadow.
VOLUME ["/srv/files", "/var/lib/nimbusfs"]
EXPOSE 8080

ENTRYPOINT ["nimbusfs"]
CMD ["serve", "-config", "/etc/nimbusfs/config.yaml"]
