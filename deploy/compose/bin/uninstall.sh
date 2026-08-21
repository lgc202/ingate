#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

KEEP_DATA=false
REMOVE_IMAGES=false
ASSUME_YES=false

usage() {
  cat <<'EOF'
Usage: uninstall.sh [OPTIONS]

Remove the current Ingate Docker Compose installation.

Options:
  --keep-data      Preserve Docker volumes for a later reinstall
  --remove-images  Remove Ingate component images after stopping containers
  --yes            Skip interactive confirmation
  -h, --help       Show this help
EOF
}

fail() {
  echo "Error: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep-data)
      KEEP_DATA=true
      shift
      ;;
    --remove-images)
      REMOVE_IMAGES=true
      shift
      ;;
    --yes)
      ASSUME_YES=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

[[ -f "$ROOT/VERSION" && -f "$ROOT/compose.yaml" ]] || fail "invalid Ingate installation: $ROOT"
[[ "$ROOT" != "/" && "$ROOT" != "${HOME:-}" ]] || fail "refusing to remove unsafe installation path: $ROOT"

VERSION="$(cat "$ROOT/VERSION")"

echo "Ingate $VERSION will be removed from $ROOT"
if [[ "$KEEP_DATA" == true ]]; then
  echo "Docker volumes will be preserved."
else
  echo "All Gateway configuration and analytics data will be permanently deleted."
fi
if [[ "$REMOVE_IMAGES" == true ]]; then
  echo "Downloaded Ingate component images will also be removed when not used elsewhere."
fi

if [[ "$ASSUME_YES" != true ]]; then
  [[ -t 0 ]] || fail "interactive confirmation is required; rerun with --yes"
  read -r -p 'Type "uninstall" to continue: ' confirmation
  [[ "$confirmation" == "uninstall" ]] || fail "uninstall cancelled"
fi

images=()
if [[ "$REMOVE_IMAGES" == true ]]; then
  while IFS= read -r image; do
    case "${image##*/}" in
      ingate-*) images+=("$image") ;;
    esac
  done < <("${COMPOSE[@]}" config --images | sort -u)
fi

down_args=(down --remove-orphans)
if [[ "$KEEP_DATA" != true ]]; then
  down_args+=(--volumes)
fi
"${COMPOSE[@]}" "${down_args[@]}"

if [[ "$REMOVE_IMAGES" == true ]]; then
  for image in "${images[@]}"; do
    if ! docker image rm "$image" >/dev/null; then
      echo "Warning: image is still in use and was retained: $image" >&2
    fi
  done
fi

cd /
rm -rf -- "$ROOT"

echo "Ingate $VERSION was uninstalled successfully."
if [[ "$KEEP_DATA" == true ]]; then
  echo "Docker volumes were preserved and can be reused by reinstalling with the same Compose project name."
fi
