# gordyclark.com build recipes.
# Everything here assumes the `nix develop` shell (go + d2 on PATH).

# Default: list recipes.
default:
    @just --list

# Build the static site into ./static.
build:
    go run ./cmd/render

# Build, then serve ./static locally at http://localhost:8000.
# Serves static/ as the web root so the absolute /fonts, /style.*.css and
# /essays/ paths resolve (opening the files directly over file:// would not).
preview: build
    @echo "Serving http://localhost:8000  (Ctrl-C to stop)"
    python3 -m http.server 8000 --directory static

# Hydrate link-preview metadata for one or more markdown files.
# Usage: just hydrate content/essays/some-post.md
hydrate +files:
    go run ./cmd/hydrate {{files}}

# Run the Go test suite.
test:
    go test ./...

# One-time browser login for wrangler (OAuth). No tokens or secrets to store;
# the session is cached under ~/.config/.wrangler and reused by `deploy-api`.
login:
    wrangler login

# Build, then upload ./static to R2 using wrangler (OAuth — no S3 keys).
# This is the token-free deploy path. Sets per-file Content-Type and
# Cache-Control. Run `just login` once first. See scripts/deploy-r2.sh.
deploy-api: build
    ./scripts/deploy-r2.sh

# Build, then sync ./static to R2 via rclone (needs S3 keys in an `r2` remote).
# Use this for a guaranteed-clean mirror (rclone sync deletes stale objects).
# Credentials live in ~/.config/rclone/rclone.conf, never committed.
deploy: build
    rclone sync static/ r2:gordyclark-com

# Remove build output and caches.
clean:
    /bin/rm -rf static .cache
