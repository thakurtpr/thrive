#!/usr/bin/env bash
set -euo pipefail

ARCH=${1:-arm64}
VFKIT_VERSION=0.5.1
OUT=thrive-vm-darwin-${ARCH}.tar.gz
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

echo "==> Building thrived daemon image for linux/${ARCH}"
make build-linux

docker build -f scripts/Dockerfile.thrived \
  --platform linux/${ARCH} \
  -t thrive-daemon:local .

echo "==> Running LinuxKit build"
linuxkit build \
  --arch ${ARCH} \
  --format kernel+initrd \
  --name thrive-vm \
  scripts/thrive-vm.yml

# linuxkit outputs: thrive-vm-kernel, thrive-vm-initrd.img
mv thrive-vm-kernel        $TMP/kernel
mv thrive-vm-initrd.img    $TMP/initrd.img

echo "==> Downloading vfkit v${VFKIT_VERSION} (self-contained bundle)"
curl -fsSL \
  "https://github.com/crc-org/vfkit/releases/download/v${VFKIT_VERSION}/vfkit-darwin-${ARCH}" \
  -o $TMP/vfkit
chmod +x $TMP/vfkit

echo "==> Packaging ${OUT}"
tar -czf ${OUT} -C $TMP kernel initrd.img vfkit
echo "Built: ${OUT} ($(du -sh ${OUT} | cut -f1))"

if command -v gh &>/dev/null; then
  echo "==> Uploading to GitHub releases"
  gh release upload latest ${OUT} --clobber
fi
