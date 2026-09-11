#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $(basename "$0") <git-tag> <stage|reveal> [--dry-run]" >&2
  exit 1
}

[ $# -ge 2 ] || usage

RAW_TAG="$1"
PHASE="$2"
case "$PHASE" in
  stage|reveal) ;;
  *) usage ;;
esac

DRY_RUN=0
if [ "${3:-}" = "--dry-run" ]; then
  DRY_RUN=1
fi

VERSION="${RAW_TAG#v}"
case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "npm-publish: '$RAW_TAG' does not look like a semver tag" >&2
    exit 1
    ;;
esac

# WHY: staging under a fixed non-default tag keeps every package unreachable via its
# conventional install path until the reveal phase publishes the main package under the
# real tag; the platform packages are pulled by exact version pin, not by tag, so which
# tag they carry never matters to an installer.
STAGE_TAG="next"

DIST_TAG="latest"
case "$VERSION" in
  *-*) DIST_TAG="next" ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
NPM_DIR="$ROOT_DIR/npm"
MAIN_DIR="$NPM_DIR/weaviate-cloud"

set_version() {
  jq --arg v "$VERSION" '.version = $v' "$1/package.json" > "$1/package.json.tmp"
  mv "$1/package.json.tmp" "$1/package.json"
}

bump_main_version() {
  jq --arg v "$VERSION" '
    .version = $v
    | .optionalDependencies |= with_entries(.value = $v)
  ' "$MAIN_DIR/package.json" > "$MAIN_DIR/package.json.tmp"
  mv "$MAIN_DIR/package.json.tmp" "$MAIN_DIR/package.json"
}

copy_binary() {
  platform="$1"
  goos="$2"
  goarch="$3"
  binary_name="wcloud"
  [ "$goos" = "windows" ] && binary_name="wcloud.exe"

  bin_dir=""
  for d in "$DIST_DIR"/wcloud_"${goos}"_"${goarch}"*/; do
    [ -d "$d" ] || continue
    if [ -n "$bin_dir" ]; then
      echo "npm-publish: ambiguous build output for ${goos}/${goarch} under $DIST_DIR" >&2
      exit 1
    fi
    bin_dir="$d"
  done
  if [ -z "$bin_dir" ]; then
    echo "npm-publish: no build output for ${goos}/${goarch} under $DIST_DIR" >&2
    exit 1
  fi

  pkg_dir="$NPM_DIR/weaviate-cloud-${platform}"
  cp "${bin_dir}${binary_name}" "$pkg_dir/$binary_name"
  [ "$goos" = "windows" ] || chmod +x "$pkg_dir/$binary_name"
}

already_published() {
  dir="$1"
  name=$(jq -r .name "$dir/package.json")
  existing=$(npm view "${name}@${VERSION}" version 2>/dev/null || true)
  [ "$existing" = "$VERSION" ]
}

# WHY: a published version can never be republished, so a retry after a downstream flip
# failure (e.g. the homebrew merge) must not re-attempt an npm publish that already
# succeeded — skipping here is what makes the flip step safe to just re-run.
publish_pkg() {
  dir="$1"
  tag="$2"
  if already_published "$dir"; then
    echo "npm-publish: $(jq -r .name "$dir/package.json")@${VERSION} already published, skipping"
    return
  fi
  args=(--access public --tag "$tag")
  [ "$DRY_RUN" -eq 1 ] && args+=(--dry-run)
  (cd "$dir" && npm publish "${args[@]}")
}

PLATFORMS="darwin-arm64:darwin:arm64 darwin-x64:darwin:amd64 linux-arm64:linux:arm64 linux-x64:linux:amd64 win32-x64:windows:amd64"

bump_main_version

if [ "$PHASE" = "stage" ]; then
  for entry in $PLATFORMS; do
    platform="${entry%%:*}"
    rest="${entry#*:}"
    goos="${rest%%:*}"
    goarch="${rest#*:}"

    copy_binary "$platform" "$goos" "$goarch"
    set_version "$NPM_DIR/weaviate-cloud-${platform}"
    publish_pkg "$NPM_DIR/weaviate-cloud-${platform}" "$STAGE_TAG"
  done
  echo "npm-publish: staged version $VERSION (platform packages published under $STAGE_TAG)"
else
  publish_pkg "$MAIN_DIR" "$DIST_TAG"
  echo "npm-publish: published version $VERSION under dist-tag $DIST_TAG"
fi
