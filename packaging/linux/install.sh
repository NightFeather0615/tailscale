#!/bin/sh
#
# Installs or updates this fork's Tailscale build on a Linux machine.
#
#   curl -fsSL https://raw.githubusercontent.com/NightFeather0615/tailscale/fork/packaging/linux/install.sh | sudo sh
#
# Run from an unpacked release archive, it installs the files next to it.
# Otherwise it downloads the latest release, or the one named by VERSION
# (such as VERSION=1.106.1-nf.1), and checks it against the release's
# SHA256SUMS file.
#
# The binaries go to /usr/local, so a tailscale package from the
# distribution stays installed and untouched. If that package's systemd unit
# exists, a drop-in points it at the fork build; otherwise the script
# installs a unit of its own.

set -eu

repo="NightFeather0615/tailscale"
bin_dir="/usr/local/bin"
sbin_dir="/usr/local/sbin"
dropin_dir="/etc/systemd/system/tailscaled.service.d"

die() {
  echo "install.sh: $*" >&2
  exit 1
}

[ "$(id -u)" -eq 0 ] || die "run this as root, for example with sudo"

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "no build for $(uname -m); releases cover amd64 and arm64" ;;
esac

# Piped into sh, $0 names the shell, not this file.
src=""
case "$0" in
  install.sh | */install.sh) src=$(cd "$(dirname "$0")" && pwd) ;;
esac
tmp=""
cleanup() {
  [ -z "${tmp}" ] || rm -rf "${tmp}"
}
trap cleanup EXIT

if [ -z "${src}" ] || [ ! -f "${src}/tailscaled" ]; then
  command -v curl > /dev/null || die "curl is needed to download the release"
  command -v sha256sum > /dev/null || die "sha256sum is needed to check the download"

  version="${VERSION:-}"
  if [ -z "${version}" ]; then
    version=$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
      | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' | head -n 1)
    [ -n "${version}" ] || die "could not find the latest release of ${repo}"
  fi

  name="tailscale-nf_${version}_linux_${arch}"
  url="https://github.com/${repo}/releases/download/v${version}"
  tmp=$(mktemp -d)
  echo "Downloading ${name}.tgz"
  curl -fsSL -o "${tmp}/${name}.tgz" "${url}/${name}.tgz"
  curl -fsSL -o "${tmp}/SHA256SUMS" "${url}/SHA256SUMS"
  (cd "${tmp}" && grep " ${name}.tgz\$" SHA256SUMS | sha256sum -c - > /dev/null) \
    || die "${name}.tgz does not match SHA256SUMS"
  tar -C "${tmp}" -xzf "${tmp}/${name}.tgz"
  src="${tmp}/${name}"
fi

# install unlinks the old file first, so this works while tailscaled runs.
install -d "${bin_dir}" "${sbin_dir}"
install -m 0755 "${src}/tailscale" "${bin_dir}/tailscale"
install -m 0755 "${src}/tailscaled" "${sbin_dir}/tailscaled"
echo "Installed $("${bin_dir}/tailscale" version | head -n 1) to ${bin_dir} and ${sbin_dir}"

if ! command -v systemctl > /dev/null || [ ! -d /run/systemd/system ]; then
  echo "No systemd here: start ${sbin_dir}/tailscaled yourself."
  exit 0
fi

packaged_unit=""
for dir in /usr/lib/systemd/system /lib/systemd/system; do
  if [ -f "${dir}/tailscaled.service" ]; then
    packaged_unit="${dir}/tailscaled.service"
    break
  fi
done

if [ -n "${packaged_unit}" ]; then
  # Keep the packaged unit and its settings; only swap the binary it runs.
  install -d "${dropin_dir}"
  cat > "${dropin_dir}/fork.conf" << EOF
# Run the fork build (github.com/${repo}) in place of the packaged
# tailscaled. Remove this file and run "systemctl daemon-reload" to go back.
[Service]
ExecStart=
ExecStart=${sbin_dir}/tailscaled --state=/var/lib/tailscale/tailscaled.state --socket=/run/tailscale/tailscaled.sock --port=\${PORT} \$FLAGS
ExecStopPost=
ExecStopPost=${sbin_dir}/tailscaled --cleanup
EOF
else
  install -m 0644 "${src}/systemd/tailscaled.service" /etc/systemd/system/tailscaled.service
  if [ ! -f /etc/default/tailscaled ]; then
    install -D -m 0644 "${src}/systemd/tailscaled.defaults" /etc/default/tailscaled
  fi
fi

systemctl daemon-reload
systemctl enable tailscaled > /dev/null 2>&1 || true
systemctl restart tailscaled
echo "tailscaled is $(systemctl is-active tailscaled)."

if [ -x /usr/bin/tailscale ]; then
  echo "Note: /usr/bin/tailscale from the tailscale package is still there."
  echo "\"sudo tailscale\" may run it; use ${bin_dir}/tailscale to be sure."
fi
echo "To join a tailnet: tailscale up --login-server https://<your control server>"
