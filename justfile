# gordyclark.com build recipes.
# Everything here assumes the `nix develop` shell (go + d2 on PATH).
#
# Deploys are not run from here: Cloudflare builds and publishes the site on
# every push to main. See the README's Deploy section.

# Default: list recipes.
default:
    @just --list

# Build the static site into ./static.
build:
    go run ./cmd/render

# Build, then serve ./static locally at http://localhost:8000.
# Serves static/ as the web root so the absolute /fonts, /style.*.css and
# /essays/ paths resolve (opening the files directly over file:// would not).
# cmd/preview mirrors how Cloudflare serves the deployed Worker: directory URLs
# resolve to index.html, unmatched paths get the site's own 404 page, and the
# generated _headers rules are applied.
preview: build
    go run ./cmd/preview

# Hydrate link-preview metadata for one or more markdown files.
# Usage: just hydrate content/essays/some-post.md
hydrate +files:
    go run ./cmd/hydrate {{files}}

# Run the Go test suite.
test:
    go test ./...

# Remove build output and caches.
clean:
    /bin/rm -rf static .cache
