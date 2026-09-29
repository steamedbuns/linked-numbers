#!/usr/bin/env bash
# Container entrypoint (runs as root). Aligns the in-container `docker` group
# with the GID of the host's mounted socket so the `vscode` user can use
# docker without sudo. Only /etc/group inside the container is changed.
set -euo pipefail

sock=/var/run/docker.sock
if [[ -S "$sock" ]]; then
  gid="$(stat -c %g "$sock")"
  if [[ "$gid" != 0 ]]; then
    if getent group docker >/dev/null; then
      [[ "$(getent group docker | cut -d: -f3)" == "$gid" ]] || groupmod -o -g "$gid" docker
    else
      groupadd -o -g "$gid" docker
    fi
    id -nG vscode | grep -qw docker || usermod -aG docker vscode
  fi
fi

exec "$@"
