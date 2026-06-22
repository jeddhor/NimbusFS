# NimbusFS

A self-contained, Linux-native web file browser. Authentication and
authorization are both delegated to the host: login goes through PAM, and
every file operation runs impersonating the authenticated Linux user, so
real `ls -la` permissions and ownership are the only access control — no
separate user database to administer.

This is the v1 MVP slice: PAM login, SSH public-key login, reverse-proxy
header login, sandboxed file browsing (list, upload, download, rename,
move, copy, delete, create), image/video/audio/pdf/text preview,
sqlite-backed sessions, CSRF protection, and audit logging. Sharing,
search, and thumbnails are not yet built.

## Reverse proxy header login

With `auth.proxy_auth: true`, nimbusfs trusts an `X-Remote-User` header set
by a fronting reverse proxy that's already authenticated the request (e.g.
Authelia, Keycloak, oauth2-proxy — see `deploy/nginx.conf.example` and
`deploy/apache.conf.example`). The named account still must be a real Linux
user, and file access is still governed entirely by that user's real
permissions. **nimbusfs has no way to verify a request actually passed
through your proxy** — it's the operator's responsibility to make sure the
proxy strips any client-supplied `X-Remote-User` before setting its own,
and that nimbusfs only listens where the proxy can reach it (e.g.
`127.0.0.1`, per `server.listen`).

## SSH public key login

Browsers can't read or sign with an arbitrary local SSH key, so this isn't
literal in-browser WebAuthn — it's a device flow like `gh auth login`:

1. On the login page, enter your username and click "Sign in with SSH key".
   The browser shows a short code.
2. On the machine holding your key, run:
   ```
   nimbusfs ssh-login --server https://your-server --code <code>
   ```
   It signs the challenge via your running `ssh-agent` (or `-key
   /path/to/key` for a specific file) and submits it.
3. The browser tab polling in the background picks up the approval and logs in.

The server verifies the signature against `~/.ssh/authorized_keys` for that
account (ed25519/RSA/ECDSA all work, since Go's `ssh.Signer`/`PublicKey`
handle the wire formats natively) and applies sshd's StrictModes checks —
the user's home dir, `.ssh`, and `authorized_keys` must be owned by that
user or root and not group/world-writable. Enable it with `auth.ssh_keys:
true` in the config.

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
