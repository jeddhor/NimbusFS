#!/usr/bin/env bash
# Starts the nimbusfs Docker container. Run with --help for usage.
set -euo pipefail

FILESERVER_ROOT=""
DATA_DIR="./nimbusfs-data"
CONFIG_PATH="./nimbusfs-config.yaml"
PORT="8080"
CONTAINER_NAME="nimbusfs"
IMAGE="nimbusfs"
USE_PAM=false
DETACH=true

usage() {
    cat <<'EOF'
Usage: ./docker-run.sh --fileserver-root="/srv/files" [options]

Required:
  --fileserver-root=PATH   Host directory to serve, mounted at /srv/files.

Options:
  --data-dir=PATH    Host dir for sqlite db / thumbnail cache, mounted at
                      /var/lib/nimbusfs. (default: ./nimbusfs-data)
  --config=PATH       Config file mounted read-only at
                      /etc/nimbusfs/config.yaml; generated with defaults
                      if missing. (default: ./nimbusfs-config.yaml)
  --port=PORT         Host port to publish. (default: 8080)
  --name=NAME         Container name. (default: nimbusfs)
  --image=IMAGE       Image tag to run. (default: nimbusfs)
  --pam               Generate a PAM-auth config instead of proxy_auth.
                      Only affects a freshly generated config — the host's
                      /etc/passwd, /etc/group, /etc/shadow, and /etc/pam.d
                      are always bind-mounted read-only (harmless if PAM
                      auth ends up disabled in config.yaml; needed if it's
                      ever turned on later without re-running this script).
  --foreground        Run attached instead of detached.

See the "Docker" section in README.md for auth-mode tradeoffs in a container.
EOF
}

for arg in "$@"; do
    case "$arg" in
    --fileserver-root=*) FILESERVER_ROOT="${arg#*=}" ;;
    --data-dir=*) DATA_DIR="${arg#*=}" ;;
    --config=*) CONFIG_PATH="${arg#*=}" ;;
    --port=*) PORT="${arg#*=}" ;;
    --name=*) CONTAINER_NAME="${arg#*=}" ;;
    --image=*) IMAGE="${arg#*=}" ;;
    --pam) USE_PAM=true ;;
    --foreground) DETACH=false ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        echo "error: unknown argument: $arg" >&2
        echo "run with --help for usage" >&2
        exit 1
        ;;
    esac
done

if [[ -z "$FILESERVER_ROOT" ]]; then
    echo "error: --fileserver-root is required, e.g. --fileserver-root=\"/srv/files\"" >&2
    exit 1
fi
if [[ ! -d "$FILESERVER_ROOT" ]]; then
    echo "error: --fileserver-root \"$FILESERVER_ROOT\" does not exist or is not a directory" >&2
    exit 1
fi
FILESERVER_ROOT="$(cd "$FILESERVER_ROOT" && pwd)"

mkdir -p "$DATA_DIR"
DATA_DIR="$(cd "$DATA_DIR" && pwd)"

if [[ ! -f "$CONFIG_PATH" ]]; then
    echo "no config found at $CONFIG_PATH, generating a default one..."
    mkdir -p "$(dirname "$CONFIG_PATH")"
    if [[ "$USE_PAM" == true ]]; then
        pam_value=true
        proxy_value=false
    else
        pam_value=false
        proxy_value=true
    fi
    cat >"$CONFIG_PATH" <<EOF
server:
  listen: 0.0.0.0:8080
  behind_proxy: true
filesystem:
  root: /srv/files
auth:
  pam: $pam_value
  ssh_keys: false
  proxy_auth: $proxy_value
sharing:
  enabled: true
search:
  enabled: true
ui:
  dark_mode: true
data_dir: /var/lib/nimbusfs
EOF
    if [[ "$USE_PAM" == false ]]; then
        cat >&2 <<'EOF'
note: generated config uses auth.proxy_auth, which expects a reverse proxy
in front of nimbusfs setting X-Remote-User (see deploy/nginx.conf.example).
Re-run with --pam to use PAM auth against the host's user database instead.
EOF
    fi
fi
CONFIG_PATH="$(cd "$(dirname "$CONFIG_PATH")" && pwd)/$(basename "$CONFIG_PATH")"

docker_args=(
    --name "$CONTAINER_NAME"
    -p "${PORT}:8080"
    -v "${CONFIG_PATH}:/etc/nimbusfs/config.yaml:ro"
    -v "${FILESERVER_ROOT}:/srv/files"
    -v "${DATA_DIR}:/var/lib/nimbusfs"
    # Always mounted, regardless of --pam: /etc/passwd and /etc/group make
    # file ownership show real host usernames instead of bare uids even
    # under proxy_auth/ssh_keys, and /etc/shadow + /etc/pam.d are needed the
    # moment auth.pam is true in config.yaml. Recreating the container
    # without these (e.g. a plain `docker run` after `docker rm`, omitting
    # --pam) is what silently breaks PAM login against the image's own
    # bundled shadow database instead of the host's.
    -v /etc/passwd:/etc/passwd:ro
    -v /etc/group:/etc/group:ro
    -v /etc/shadow:/etc/shadow:ro
    -v /etc/pam.d:/etc/pam.d:ro
)

if [[ "$DETACH" == true ]]; then
    docker_args=(-d "${docker_args[@]}")
fi

echo "fileserver root: $FILESERVER_ROOT -> /srv/files"
echo "data dir:        $DATA_DIR -> /var/lib/nimbusfs"
echo "config:          $CONFIG_PATH -> /etc/nimbusfs/config.yaml"
echo "+ docker run ${docker_args[*]} $IMAGE"
echo

docker run "${docker_args[@]}" "$IMAGE"

if [[ "$DETACH" == true ]]; then
    echo
    echo "started. http://localhost:${PORT}"
    echo "logs:  docker logs -f $CONTAINER_NAME"
    echo "stop:  docker stop $CONTAINER_NAME && docker rm $CONTAINER_NAME"
fi
