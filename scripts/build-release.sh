#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-1.0.0}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must match X.Y.Z: $VERSION" >&2
  exit 1
fi
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
BACKEND_DIR="$ROOT_DIR/backend"
DIST_NAME="auth_pro-full-v$VERSION"
RELEASE_ROOT="$ROOT_DIR/release"
PACKAGE_DIR="$RELEASE_ROOT/$DIST_NAME"
PACKAGES_DIR="$RELEASE_ROOT/packages"
PACKAGE_PATH="$PACKAGES_DIR/$DIST_NAME.tar.gz"
LATEST_PATH="$PACKAGES_DIR/latest.json"
RELEASES_PATH="$PACKAGES_DIR/releases.json"
GITHUB_REPOSITORY="${AUTO_PRO_GITHUB_REPOSITORY:-cy70923167/auth_pro}"
UPDATE_PACKAGE_BASE_URL="${AUTO_PRO_UPDATE_PACKAGE_BASE_URL:-https://github.com/$GITHUB_REPOSITORY/releases/download/v$VERSION}"
UPDATE_RELEASES_URL="${AUTO_PRO_UPDATE_RELEASES_URL:-https://github.com/$GITHUB_REPOSITORY/releases/latest/download/releases.json}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
LDFLAGS="-s -w -X auto_pro/config.AppVersion=$VERSION -X auto_pro/config.BuildTime=$BUILD_TIME"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
mkdir -p "$GOCACHE"

case "$PACKAGE_DIR" in
  "$ROOT_DIR"/*) ;;
  *) echo "Invalid package directory: $PACKAGE_DIR" >&2; exit 1 ;;
esac

printf '[1/5] Building frontend...\n'
VITE_VERSION="$VERSION" pnpm -C "$FRONTEND_DIR" run build

test -f "$FRONTEND_DIR/dist/index.html"
test -f "$FRONTEND_DIR/dist/version.json"

printf '[2/5] Preparing package directories...\n'
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR/backend" "$PACKAGES_DIR" "$BACKEND_DIR/static"
rm -rf "$BACKEND_DIR/static"/*
cp -R "$FRONTEND_DIR/dist"/. "$PACKAGE_DIR"/
cp -R "$FRONTEND_DIR/dist"/. "$BACKEND_DIR/static"/

printf '[3/5] Building Linux amd64 backend...\n'
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go -C "$BACKEND_DIR" build -trimpath -ldflags "$LDFLAGS" -o "$BACKEND_DIR/auto_pro_linux_amd64" .
cp "$BACKEND_DIR/auto_pro_linux_amd64" "$PACKAGE_DIR/backend/auth_pro"

printf '[4/5] Writing manifest...\n'
printf '{\n  "version": "%s",\n  "frontendDir": ".",\n  "backendFile": "backend/auth_pro",\n  "requiredFiles": []\n}\n' "$VERSION" > "$PACKAGE_DIR/manifest.json"

printf '[5/5] Creating tar.gz package and latest.json...\n'
rm -f "$PACKAGE_PATH"
tar -czf "$PACKAGE_PATH" -C "$PACKAGE_DIR" .

if command -v shasum >/dev/null 2>&1; then
  PACKAGE_SHA256="$(shasum -a 256 "$PACKAGE_PATH" | cut -d ' ' -f 1)"
else
  PACKAGE_SHA256="$(sha256sum "$PACKAGE_PATH" | cut -d ' ' -f 1)"
fi
if PACKAGE_SIZE="$(stat -f '%z' "$PACKAGE_PATH" 2>/dev/null)"; then
  :
else
  PACKAGE_SIZE="$(stat -c '%s' "$PACKAGE_PATH")"
fi
PACKAGE_URL="${UPDATE_PACKAGE_BASE_URL%/}/$DIST_NAME.tar.gz"
node "$ROOT_DIR/scripts/write-release-manifests.mjs" \
  "$LATEST_PATH" \
  "$RELEASES_PATH" \
  "$VERSION" \
  "$BUILD_TIME" \
  "$DIST_NAME.tar.gz" \
  "$PACKAGE_URL" \
  "$PACKAGE_SHA256" \
  "$PACKAGE_SIZE" \
  "$UPDATE_RELEASES_URL"

printf '\nRelease package: %s\n' "$PACKAGE_PATH"
printf 'Latest manifest: %s\n' "$LATEST_PATH"
printf 'Release history: %s\n' "$RELEASES_PATH"
printf 'SHA256: %s\n' "$PACKAGE_SHA256"
printf 'Size: %s bytes\n' "$PACKAGE_SIZE"