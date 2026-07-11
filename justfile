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

# Build, then sync ./static to the Cloudflare R2 bucket.
# Credentials come from the environment (see README / rclone config), never committed:
#   rclone remote "r2" configured for the R2 S3 endpoint, or AWS_* env vars for aws-cli.
deploy: build
    rclone sync static/ r2:gordyclark-com

# Remove build output and caches.
clean:
    /bin/rm -rf static .cache
