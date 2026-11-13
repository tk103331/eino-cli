#!/usr/bin/env bash
# Update the Homebrew tap formula for a published eino-cli release.
# Required env: VERSION (e.g. v0.1.0), GH_TOKEN (PAT with push access to the tap).
set -euo pipefail

VERSION_TAG="${VERSION:?VERSION is required}"
VERSION_NO_V="${VERSION_TAG#v}"
TAP_REPO="${TAP_REPO:-tk103331/homebrew-tap}"
BASE="https://github.com/tk103331/eino-cli/releases/download/${VERSION_TAG}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

curl -fsSL "${BASE}/checksums.txt" -o "${workdir}/checksums.txt"

sha_for() {
  awk -v f="$1" '$2 == f { print $1 }' "${workdir}/checksums.txt"
}

DARWIN_ARM64="$(sha_for "eino_${VERSION_NO_V}_darwin_arm64.tar.gz")"
DARWIN_AMD64="$(sha_for "eino_${VERSION_NO_V}_darwin_amd64.tar.gz")"
LINUX_ARM64="$(sha_for "eino_${VERSION_NO_V}_linux_arm64.tar.gz")"
LINUX_AMD64="$(sha_for "eino_${VERSION_NO_V}_linux_amd64.tar.gz")"

if [ -z "${DARWIN_ARM64}" ] || [ -z "${DARWIN_AMD64}" ] || [ -z "${LINUX_ARM64}" ] || [ -z "${LINUX_AMD64}" ]; then
  echo "missing checksums in ${BASE}/checksums.txt" >&2
  cat "${workdir}/checksums.txt" >&2
  exit 1
fi

sed \
  -e "s/__VERSION__/${VERSION_NO_V}/" \
  -e "s/__SHA_EINO_DARWIN_ARM64__/${DARWIN_ARM64}/" \
  -e "s/__SHA_EINO_DARWIN_AMD64__/${DARWIN_AMD64}/" \
  -e "s/__SHA_EINO_LINUX_ARM64__/${LINUX_ARM64}/" \
  -e "s/__SHA_EINO_LINUX_AMD64__/${LINUX_AMD64}/" \
  Formula/eino-cli.rb > "${workdir}/eino-cli.rb"

git clone --depth 1 "https://x-access-token:${GH_TOKEN}@github.com/${TAP_REPO}.git" "${workdir}/tap"
mkdir -p "${workdir}/tap/Formula"
cp "${workdir}/eino-cli.rb" "${workdir}/tap/Formula/eino-cli.rb"

cd "${workdir}/tap"
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add Formula/eino-cli.rb
if git diff --cached --quiet; then
  echo "formula unchanged for ${VERSION_NO_V}"
  exit 0
fi
git commit -m "eino-cli ${VERSION_NO_V}"
git push
