#!/usr/bin/env bash
#
# Builds the release files from a patched tailscale.com checkout.
#
#   scripts/build.sh <source dir> <output dir> <release>
#
# <release> is the name of the release, such as 1.106.1-nf.1. The source
# has to be a git checkout: the version stamp and the patch file come from
# its history.

set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <source dir> <output dir> <release>" >&2
  exit 2
fi

src=$(cd "$1" && pwd)
mkdir -p "$2"
out=$(cd "$2" && pwd)
release=$3
here=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# The builds leave out the self-updater. It installs the official packages,
# which would replace this build.
readonly tags="ts_omit_clientupdate"

readonly wintun_version="0.14.1"
readonly wintun_sha256="07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"

cd "${src}"

go="./tool/go"
if [[ "${OS:-}" == "Windows_NT" ]]; then
  go="./tool/go.exe"
fi

eval "$("${go}" run ./cmd/mkversion | tr -d '\r')"
ldflags="-s -w"
ldflags+=" -X tailscale.com/version.longStamp=${VERSION_LONG}"
ldflags+=" -X tailscale.com/version.shortStamp=${VERSION_SHORT}"

# Archives get the date of the source commit, so that a rebuild of the same
# source gives the same files.
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)
export SOURCE_DATE_EPOCH
stamp=$(date -u -d "@${SOURCE_DATE_EPOCH}" +%Y%m%d%H%M.%S)

work=$(mktemp -d)
trap 'rm -rf "${work}"' EXIT

build() {
  local os=$1 arch=$2 dest=$3
  shift 3
  GOOS="${os}" GOARCH="${arch}" CGO_ENABLED=0 "${go}" build \
    -trimpath -tags "${tags}" -ldflags "${ldflags}" -o "${dest}/" "$@"
}

for arch in amd64 arm64; do
  name="tailscale-nf_${release}_linux_${arch}"
  dir="${work}/${name}"
  mkdir -p "${dir}/systemd"
  echo "Building ${name}"
  build linux "${arch}" "${dir}" ./cmd/tailscaled ./cmd/tailscale
  sed 's|/usr/sbin/tailscaled|/usr/local/sbin/tailscaled|g' \
    cmd/tailscaled/tailscaled.service > "${dir}/systemd/tailscaled.service"
  cp cmd/tailscaled/tailscaled.defaults "${dir}/systemd/"
  cp "${here}/packaging/linux/install.sh" "${dir}/"
  chmod 0755 "${dir}/install.sh" "${dir}/tailscale" "${dir}/tailscaled"
  cp LICENSE "${dir}/"
  tar -C "${work}" --sort=name --owner=0 --group=0 --numeric-owner \
    --mtime="@${SOURCE_DATE_EPOCH}" -czf "${out}/${name}.tgz" "${name}"
done

name="tailscale-nf_${release}_windows_amd64"
dir="${work}/${name}"
mkdir -p "${dir}"
echo "Building ${name}"
build windows amd64 "${dir}" ./cmd/tailscaled ./cmd/tailscale
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "${go}" build \
  -trimpath -tags "${tags}" -ldflags "${ldflags} -H=windowsgui" \
  -o "${dir}/tailscale-systray.exe" ./cmd/systray

# tailscaled loads wintun.dll from its own directory. The official installer
# ships the same file.
curl -fsSL -o "${work}/wintun.zip" \
  "https://www.wintun.net/builds/wintun-${wintun_version}.zip"
echo "${wintun_sha256}  ${work}/wintun.zip" | sha256sum -c - > /dev/null
unzip -q "${work}/wintun.zip" -d "${work}"
cp "${work}/wintun/bin/amd64/wintun.dll" "${dir}/"
cp "${work}/wintun/LICENSE.txt" "${dir}/wintun-LICENSE.txt"

cp "${here}/packaging/windows/install.ps1" "${dir}/"
cp LICENSE "${dir}/LICENSE.txt"
find "${dir}" -exec touch -t "${stamp}" {} +
(cd "${work}" && find "${name}" -type f | LC_ALL=C sort | zip -q -X -@ "${out}/${name}.zip")

# The changes this build makes to the upstream release, as one mailbox file
# that "git am" applies.
git format-patch --stdout "v${VERSION_SHORT}..HEAD" > "${out}/tailscale-nf_${release}.patch"

(cd "${out}" && sha256sum -- *.tgz *.zip *.patch > SHA256SUMS)

echo "Built ${VERSION_LONG} as ${release}:"
ls -l "${out}"
