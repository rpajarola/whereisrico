#!/usr/bin/env bash
# Builds a signed apt repository from a directory of .deb files, laid out
# for static hosting (GitHub Pages; see .github/workflows/apt-repo.yml).
#
# Requires apt-ftparchive (apt-utils) and gpg, with the signing key's
# secret key already imported.
#
# Usage:
#   GPG_KEY_ID=<fingerprint> packaging/apt-repo.sh <deb-dir> <out-dir>
#
# <out-dir> is wiped and recreated. Users add the result with:
#   deb [signed-by=/usr/share/keyrings/whereisrico.gpg] <url> stable main
set -euo pipefail

if [ $# -ne 2 ] || [ -z "${GPG_KEY_ID:-}" ]; then
  echo "usage: GPG_KEY_ID=<fingerprint> $0 <deb-dir> <out-dir>" >&2
  exit 2
fi
DEB_DIR="$(cd "$1" && pwd)"
OUT="$2"

SUITE=stable
COMPONENT=main
ARCHES="amd64 arm64"

shopt -s nullglob
debs=("$DEB_DIR"/*.deb)
if [ ${#debs[@]} -eq 0 ]; then
  echo "apt-repo.sh: no .deb files in $DEB_DIR" >&2
  exit 1
fi

rm -rf "$OUT"
mkdir -p "$OUT/pool/$COMPONENT"
cp "${debs[@]}" "$OUT/pool/$COMPONENT/"
cd "$OUT"

for arch in $ARCHES; do
  dir="dists/$SUITE/$COMPONENT/binary-$arch"
  mkdir -p "$dir"
  apt-ftparchive --arch "$arch" packages "pool/$COMPONENT" > "$dir/Packages"
  gzip -9nk "$dir/Packages"
done

# Written to a temp file first: apt-ftparchive would otherwise checksum
# the half-written Release file it is generating.
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=whereisrico \
  -o APT::FTPArchive::Release::Label=whereisrico \
  -o APT::FTPArchive::Release::Suite="$SUITE" \
  -o APT::FTPArchive::Release::Codename="$SUITE" \
  -o APT::FTPArchive::Release::Architectures="$ARCHES" \
  -o APT::FTPArchive::Release::Components="$COMPONENT" \
  release "dists/$SUITE" > Release.tmp
mv Release.tmp "dists/$SUITE/Release"

gpg --batch --yes --local-user "$GPG_KEY_ID" --armor --detach-sign \
  --output "dists/$SUITE/Release.gpg" "dists/$SUITE/Release"
gpg --batch --yes --local-user "$GPG_KEY_ID" --clearsign \
  --output "dists/$SUITE/InRelease" "dists/$SUITE/Release"

# Public key, binary (for /usr/share/keyrings) and armored.
gpg --batch --export "$GPG_KEY_ID" > whereisrico.gpg
gpg --batch --armor --export "$GPG_KEY_ID" > whereisrico.asc

url="${REPO_URL:-https://rpajarola.github.io/whereisrico}"
cat > index.html <<EOF
<!doctype html>
<meta charset="utf-8">
<title>whereisrico apt repository</title>
<h1>whereisrico apt repository</h1>
<pre>
sudo curl -fsSLo /usr/share/keyrings/whereisrico.gpg $url/whereisrico.gpg
echo "deb [signed-by=/usr/share/keyrings/whereisrico.gpg] $url $SUITE $COMPONENT" \\
  | sudo tee /etc/apt/sources.list.d/whereisrico.list
sudo apt update
sudo apt install whereisrico
</pre>
<p>Source: <a href="https://github.com/rpajarola/whereisrico">github.com/rpajarola/whereisrico</a></p>
EOF

echo "apt-repo.sh: built $SUITE/$COMPONENT with ${#debs[@]} package(s) in $OUT"
