#!/usr/bin/env bash
# Upload ./static to the R2 bucket using wrangler (OAuth login — no S3 keys or
# API tokens to manage). Run `wrangler login` once; the session is reused.
#
# Sets a correct Content-Type and Cache-Control per file: HTML is served with a
# short TTL so edits appear quickly, while the content-hashed CSS and the font
# are marked immutable for a year.
#
# Note: wrangler's `r2 object` command has no `list`, so this script cannot
# sync-delete stale remote objects. In practice the only file that changes name
# between builds is the content-hashed stylesheet (style.<hash>.css); an old one
# left behind is harmless (nothing references it). For a guaranteed-clean mirror
# use `just deploy` (rclone sync, needs S3 keys) instead. To remove old
# stylesheets after a big CSS change, run:  just prune-css
set -euo pipefail

BUCKET="gordyclark-com"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATIC="$ROOT/static"

if [ ! -f "$STATIC/index.html" ]; then
  echo "error: $STATIC/index.html not found — run 'just build' first" >&2
  exit 1
fi

content_type() {
  case "$1" in
    *.html)  echo "text/html; charset=utf-8" ;;
    *.css)   echo "text/css; charset=utf-8" ;;
    *.woff2) echo "font/woff2" ;;
    *.svg)   echo "image/svg+xml" ;;
    *.js)    echo "text/javascript; charset=utf-8" ;;
    *.png)   echo "image/png" ;;
    *.jpg|*.jpeg) echo "image/jpeg" ;;
    *)       echo "application/octet-stream" ;;
  esac
}

cache_control() {
  case "$1" in
    *.html)        echo "public, max-age=60" ;;
    *.css|*.woff2) echo "public, max-age=31536000, immutable" ;;
    *)             echo "public, max-age=3600" ;;
  esac
}

echo "Uploading $STATIC -> r2://$BUCKET (wrangler, --remote)"
cd "$STATIC"

while IFS= read -r f; do
  key="${f#./}"
  ct="$(content_type "$f")"
  cc="$(cache_control "$f")"
  echo "  $key  [$ct]"
  wrangler r2 object put "$BUCKET/$key" \
    --file "$f" \
    --content-type "$ct" \
    --cache-control "$cc" \
    --remote >/dev/null
done < <(find . -type f | sort)

echo "Done. Live at https://gordyclark.com/ (allow a moment for HTML cache TTL)."
