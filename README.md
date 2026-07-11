# gordyclark.com

A static essay site: Markdown in, static HTML/CSS out. No server, no database, no
JavaScript on essay pages. Built with Go — [goldmark](https://github.com/yuin/goldmark)
for Markdown, [Chroma](https://github.com/alecthomas/chroma) for syntax highlighting,
[D2](https://d2lang.com) for diagrams — and deployed to Cloudflare R2.

## Layout

```
cmd/render      site build: content/ + templates/ + assets/ -> static/
cmd/hydrate     manual tool: fetches link-preview metadata into markdown
internal/       content parsing, margin extraction, diagrams, highlight, render
templates/      base / essay / index Go html/templates
assets/         css (concatenated per manifest.txt) + self-hosted font
content/        essays, pages, citations.yaml
static/         BUILD OUTPUT — gitignored
.cache/         diagram SVG cache, keyed by content hash — gitignored
```

## Develop

Everything runs inside the Nix dev shell, which provides Go and the pinned `d2`
binary (plus `just`, `rclone`):

```sh
nix develop           # enter the shell
just build            # render content/ -> static/
just hydrate content/essays/some-post.md   # fetch link-preview metadata (idempotent)
just test             # go test ./...
```

`nix build` produces `static/` from a clean checkout with no network access.

## Authoring

See `content/pages/colophon.md` for the rendered explanation, and the sample essay
`content/essays/2026-07-11-on-building-a-website-out-of-files.md` for every content
convention: margin asides (footnotes), citations (`[^cite:key]` resolved from
`content/citations.yaml`), link-preview chips (`{.margin}`), D2 diagrams, highlighted
code, timelines, and dialogue blocks.

External `.margin` link chips must be hydrated before they render — the build fails
loudly on an unhydrated one and prints the exact `just hydrate` command to run.

## Deploy

Two paths to the same R2 bucket (`gordyclark-com`), both from the Nix shell:

**Token-free (recommended) — wrangler + OAuth:**

```sh
just login       # one-time browser login; session cached, no secrets stored
just deploy-api  # build + upload ./static with correct Content-Type / Cache-Control
```

`deploy-api` runs `scripts/deploy-r2.sh`, uploading each file via `wrangler r2 object put`.
HTML gets a short cache TTL; the content-hashed CSS and font are marked immutable.

**S3 keys — rclone (guaranteed-clean mirror):**

```sh
just deploy      # build + rclone sync static/ r2:gordyclark-com
```

Needs an `r2` rclone remote configured against the R2 S3 endpoint
(`https://<account-id>.r2.cloudflarestorage.com`) with an R2 *Access Key ID + Secret*
(R2 → Manage R2 API Tokens). `rclone sync` deletes stale remote objects, so it's the
way to prune old hashed stylesheets. Credentials live in `~/.config/rclone/rclone.conf`,
never committed.

### Serving

The bucket is exposed via a custom domain (R2 → bucket → Settings → Public access →
Custom Domains → `gordyclark.com`). Because R2 custom domains don't auto-resolve
directory indexes, a Cloudflare **Transform Rule** (Rewrite URL) rewrites any path
ending in `/` to append `index.html`:

```
When:  URI Path  ends with  "/"
Then:  Rewrite path (dynamic) to  concat(http.request.uri.path, "index.html")
```

This makes `/`, `/essays/<slug>/`, and `/tags/<tag>/` resolve to their `index.html`.
