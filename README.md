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

`just deploy` runs the build then `rclone sync static/ r2:gordyclark-com`. Configure
an `rclone` remote named `r2` against the R2 S3-compatible endpoint; credentials come
from the environment (`rclone config` or `AWS_*` vars) and are never committed. The R2
bucket has a connected custom domain (bucket Settings → Public access → Custom Domains).
