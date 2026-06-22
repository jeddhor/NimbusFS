# NimbusFS

A self-contained, Linux-native web file browser. Authentication and
authorization are both delegated to the host: login goes through PAM, and
every file operation runs impersonating the authenticated Linux user, so
real `ls -la` permissions and ownership are the only access control — no
separate user database to administer.

This is the v1 MVP slice: PAM login, sandboxed file browsing (list, upload,
download, rename, move, copy, delete, create), image/video/audio/pdf/text
preview, sqlite-backed sessions, CSRF protection, and audit logging.
SSH-key auth, proxy auth, sharing, search, and thumbnails are not yet built.

## Build

Requires Go 1.25+, Node 20+, and `libpam0g-dev` (or equivalent) for cgo.

```
make build      # builds web/dist then the nimbusfs binary
```

This produces a single `nimbusfs` executable. It's dynamically linked
against libpam (required — PAM modules are loaded as shared objects, so
this can't be fully static), but nothing else beyond glibc.

## Run

PAM auth requires nimbusfs to start as root, since it impersonates the
authenticated Linux user for every filesystem syscall (see
`internal/fsops/identity.go`). See `deploy/systemd/nimbusfs.service` for a
unit file and `deploy/pam.d/nimbusfs` for the PAM service file (copy to
`/etc/pam.d/nimbusfs`).

```
sudo ./nimbusfs init                  # writes /etc/nimbusfs/config.yaml
sudo ./nimbusfs check-config          # validate it
sudo ./nimbusfs serve                 # start the server
```

## Config

See `internal/config/config.go` for the full schema; matches the
`server` / `filesystem` / `auth` / `sharing` / `search` / `ui` keys from
the original spec.
