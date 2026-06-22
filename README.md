# NimbusFS

A self-contained, Linux-native web file browser. Authentication and
authorization are both delegated to the host: login goes through PAM, and
every file operation runs impersonating the authenticated Linux user, so
real `ls -la` permissions and ownership are the only access control — no
separate user database to administer.

This is the full v1 slice: PAM login, SSH public-key login, reverse-proxy
header login, sandboxed file browsing (list, upload, download, rename,
move, copy, delete, create), image/video/audio/pdf/text preview,
temporary share links, indexed search, thumbnails, sqlite-backed
sessions, CSRF protection, and audit logging. Stretch goals from the
original spec (WebDAV, SFTP browsing, in-browser editing, etc.) aren't built.

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

## Share links

From the file browser, select a file or folder and click the share icon to
create a link (`/share/<token>`), with an optional expiration, password, and
access mode (view & download / view only / download only). Anyone with the
link can access it anonymously — it's a capability token, not tied to a
session. A directory share is scoped to that subtree only: `Sandbox.ResolveWithin`
re-applies the same symlink-safe resolution as the rest of the sandbox, plus
an extra check that the resolved path can't climb back out above the shared
folder. Manage your own links from the "Shared Links" button in the toolbar.

Shares can also be created from the command line:
```
nimbusfs share --mode view_only --expires 7d /srv/files/movies/clip.mp4
```
The CLI writes directly to the same sqlite database the server uses, so it
needs the same filesystem access to that database file — in practice this
means running it as whichever user runs `nimbusfs serve` (typically root),
e.g. via `sudo`. It does not need to be run on a machine with the server
actively running.

## Search

With `search.enabled: true`, an in-memory index is built at startup and
rebuilt every 5 minutes (or on demand via `POST /api/search/reindex`).
Type into the search bar next to the breadcrumbs for instant filename
filtering; wildcards work too (`*.pdf`, `report_202?.csv`).

This is a performance tradeoff worth knowing about: checking every search
hit against the real kernel permission model (the way every other
operation in nimbusfs works) would mean one impersonated syscall per
candidate file, which doesn't scale to the "100,000 files" target. Instead,
owner/group/mode bits are captured into the index at index time and
checked in Go (`fsops.CanRead`) — standard Unix owner/group/other
semantics, but not ACLs, and not parent-directory traversal permission.
Actually opening a result still goes through the real sandboxed,
impersonated path, so this can't grant access the kernel wouldn't — at
worst a stale/ACL-only-permitted result could show up and then 403 when
opened, or a result an ACL would allow could be hidden.

## Thumbnails

jpg/png/gif/webp thumbnails are generated in pure Go (decode, resize,
re-encode as JPEG) and cached on disk under `<data_dir>/thumbs`, keyed by
path + size + mtime so they invalidate automatically when a file changes.

mp4 and pdf thumbnails are opportunistic: if `ffmpeg`/`pdftoppm` are found
on PATH at startup they're used to grab a frame/page, otherwise those
types just show a generic icon instead — no hard dependency, no cgo, and
the binary still works the same either way. When used, both tools run as a
real dropped-privilege subprocess (their actual uid/gid set to the
requesting Linux user, not just the fsuid trick used elsewhere) so they
can't read anything that user couldn't read directly themselves.

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
