#!/usr/bin/env bash
# Builds .deb packages for whereisrico (amd64 and arm64 by default).
#
# Requires nfpm (https://nfpm.goreleaser.com):
#   go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
#
# Usage:
#   packaging/build.sh                 # version from git, both arches
#   VERSION=1.2.3 packaging/build.sh   # explicit version
#   ARCHES=amd64 packaging/build.sh    # just one arch
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."  # repo root

if ! command -v nfpm >/dev/null 2>&1; then
  echo "build.sh: nfpm not found on PATH -- see packaging/README.md" >&2
  exit 1
fi

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  if git describe --tags --exact-match >/dev/null 2>&1; then
    VERSION="$(git describe --tags --exact-match | sed 's/^v//')"
  else
    VERSION="0.0.0~git$(git rev-parse --short HEAD)"
  fi
fi

ARCHES="${ARCHES:-amd64 arm64}"

mkdir -p dist dist/bin
for goarch in $ARCHES; do
  echo "== building linux/$goarch binaries =="
  # Built into a fixed path (not arch-suffixed): nfpm's ${VAR} expansion
  # doesn't reach contents[].src, so nfpm.yaml points at this fixed path
  # and we rebuild+repackage once per arch instead.
  CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags="-s -w" \
    -o "dist/bin/whereisricod" ./cmd/whereisricod
  CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags="-s -w" \
    -o "dist/bin/whereisricoctl" ./cmd/whereisricoctl

  echo "== packaging linux/$goarch .deb (version $VERSION) =="
  VERSION="$VERSION" ARCH="$goarch" nfpm pkg \
    --config packaging/nfpm.yaml \
    --packager deb \
    --target "dist/whereisrico_${VERSION}_${goarch}.deb"

  # Keep a copy of the arch-specific binaries around after packaging.
  mkdir -p "dist/$goarch"
  cp dist/bin/whereisricod dist/bin/whereisricoctl "dist/$goarch/"
done
rm -rf dist/bin

echo
echo "done:"
ls -la dist/*.deb
