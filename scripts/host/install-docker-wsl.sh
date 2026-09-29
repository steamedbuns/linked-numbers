#!/usr/bin/env bash
# One-time host setup for WSL2 Ubuntu: installs Docker Engine from Docker's
# official apt repo. This is the only thing the dev environment installs on
# the host; every other tool lives in the dev container image.
set -euo pipefail

if docker info >/dev/null 2>&1; then
  echo "Docker is already installed and reachable: $(docker --version)"
  exit 0
fi

if ! grep -qsE '^\s*systemd\s*=\s*true' /etc/wsl.conf; then
  echo "WARNING: systemd is not enabled in /etc/wsl.conf. Add:" >&2
  printf '  [boot]\n  systemd=true\n' >&2
  echo "then run 'wsl --shutdown' from Windows and rerun this script." >&2
  exit 1
fi

if ! command -v docker >/dev/null; then
  . /etc/os-release
  sudo apt-get update
  sudo apt-get install -y ca-certificates curl
  sudo install -m 0755 -d /etc/apt/keyrings
  sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  sudo chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
  sudo apt-get update
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi

sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"

echo
echo "Docker installed: $(docker --version)"
if ! id -nG | grep -qw docker; then
  echo "Your shell isn't in the 'docker' group yet. Open a new WSL terminal"
  echo "(or run 'wsl --shutdown' from Windows), then check: docker run --rm hello-world"
fi
