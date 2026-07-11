# gordyclark.com

A personal essay site built the boring, durable way: **Markdown in, static HTML/CSS
out.** No server, no database, and **zero JavaScript on essay pages**. Content lives as
plain files in git; a Go program renders them to a `static/` tree that is uploaded to
Cloudflare R2 and served over a custom domain.

Built with [goldmark](https://github.com/yuin/goldmark) (Markdown),
[Chroma](https://github.com/alecthomas/chroma) (syntax highlighting),
[D2](https://d2lang.com) (diagrams), and [Vega-Lite](https://vega.github.io/vega-lite/)
via `vl_convert` (charts). The whole toolchain is pinned with Nix.

---

## Quick start

```sh
nix develop            # enter the dev shell (Go, d2, just, rclone, wrangler)
just build             # render content/ -> static/
just preview           # build + serve at http://localhost:8000
```

Everything below assumes you're inside `nix develop`.

---

## Repository layout

```
cmd/render/            site build: content/ + templates/ + assets/ -> static/
cmd/hydrate/           manual tool: fetches link-preview metadata into markdown
internal/
  content/             frontmatter + body parsing, content index, citations
  margin/              inline {.class k="v"} attribute parser + margin-item model
  diagrams/            D2 subprocess + content-hashed SVG cache
  charts/              Vega-Lite -> themed SVG via vl_convert + cache
  highlight/           Chroma wiring (classed spans, not inline styles)
  render/              the pipeline: block walk, cell pairing, CSS bundling, output
templates/             base / essay / index Go html/templates
assets/
  css/                 small single-purpose files, concatenated per manifest.txt
  fonts/               self-hosted Public Sans (variable, woff2)
content/
  essays/              the posts (one .md each)
  pages/               about/colophon source (not currently rendered — see note)
  citations.yaml       citation database, keyed by cite key
scripts/deploy-r2.sh   token-free wrangler upload of static/ to R2
static/                BUILD OUTPUT — gitignored, never hand-edited
.cache/                diagram SVG cache, keyed by content hash — gitignored
flake.nix / flake.lock pinned toolchain (Go, d2, just, rclone, wrangler)
justfile               task runner (see below)
```

---

## `just` recipes

| Recipe | What it does |
|---|---|
| `just build` | Render `content/` → `static/` via `cmd/render`. |
| `just preview` | `build`, then serve `static/` at http://localhost:8000. Serves the dir as web root so absolute paths (`/fonts`, `/style.*.css`, `/essays/`) resolve. |
| `just hydrate <file>…` | Fetch link-preview metadata for external `{.margin}` chips and write it back into the markdown. Idempotent. |
| `just test` | `go test ./...`. |
| `just login` | One-time browser OAuth for wrangler. No secrets stored. |
| `just deploy-api` | `build`, then upload `static/` to R2 via **wrangler** (OAuth — no S3 keys). The token-free deploy. |
| `just deploy` | `build`, then `rclone sync static/ r2:gordyclark-com` (needs S3 keys; does a clean sync-delete mirror). |
| `just clean` | Remove `static/` and `.cache/`. |

Run `just` with no argument to list them.

---

## How the build works

`cmd/render` walks `content/essays/*.md` and, for each essay, parses frontmatter +
body, then renders it block by block. The output for each essay is a two-column
**article grid**: the prose in a `content-cell`, and any marginalia (notes, citations,
link chips) in the paired `margin-cell` beside it. Every content block emits both cells
(even if the margin is empty) so the CSS grid rows stay aligned.

Pipeline highlights:

- **Content index first.** All essay frontmatter is loaded up front so internal chips
  and the "related" block can resolve other essays' titles/subtitles.
- **Diagrams** (` ```d2 ` blocks) are rendered by the `d2` binary and the SVG is inlined.
  Results are cached in `.cache/diagrams/<sha256>.svg`; an unchanged diagram is never
  re-rendered. A D2 failure is **fatal** (build exits non-zero) — no silent placeholder.
- **Charts** (` ```vega ` blocks) are Vega-Lite specs compiled to static SVG by
  `vl_convert` (offline, no browser). A Mocha theme is merged into each spec so charts
  match the site. Cached in `.cache/charts/<sha256>.svg`; a bad spec is fatal. Both
  diagrams and charts render as a small thumbnail with a zero-JS click-to-enlarge modal.
- **Code** (` ```go `, ` ```yaml `, …) is highlighted by Chroma into *classed* spans;
  colors live in `assets/css/components/code.css` via the same CSS variables as the rest
  of the page.
- **CSS** files are concatenated in the order listed in `assets/css/manifest.txt`, hashed,
  and written as `static/style.<hash>.css`. The `<link>` in the template uses the exact
  hashed name, so any CSS change cache-busts automatically.
- **Fail loudly.** Missing required frontmatter, an unknown citation key, an unhydrated
  external chip, or a D2 error all stop the build with a specific file:line message.

`nix build` produces the same `static/` tree from a clean checkout with **no network
access** (d2 runs locally; Go deps are vendored).

---

## Authoring essays

Create a file in `content/essays/`, e.g. `content/essays/2026-08-01-my-post.md`:

```yaml
---
title: "My Post"
subtitle: "A one-line dek that also becomes the internal-chip description."
slug: my-post
date: 2026-08-01
tags: [essays]
status: finished        # or `draft` — drafts render but are excluded from listings
reading_time_override: null   # null = compute from word count (~230 wpm)
---
```

Required frontmatter: `title`, `slug`, `date`. The rest have defaults.

### Content conventions

- **Margin aside** — a normal Markdown footnote. The note body renders in the margin:
  ```md
  A claim that needs a caveat.[^my-note]

  [^my-note]: The caveat, shown as a margin note.
  ```
- **Citation** — a footnote whose label is `cite:<key>`, resolved from
  `content/citations.yaml` (write **no** definition line for it):
  ```md
  Static generators converge on a few shapes.[^cite:gwern-design]
  ```
- **External link chip** — a `{.margin}` link. Must be **hydrated** before it renders
  (see below):
  ```md
  [D2](https://d2lang.com){.margin domain="d2lang.com" title="…" desc="…"}
  ```
- **Internal link chip** — a `{.margin}` link to another essay; resolves from that
  essay's frontmatter every build, so it never goes stale (no hydration needed):
  ```md
  [why boring tech](/essays/boring-technology-at-workmind.md){.margin}
  ```
- **Diagram** — a ` ```d2 ` fenced block (flowcharts, architecture, sequence, …).
- **Chart** — a ` ```vega ` fenced block containing a [Vega-Lite](https://vega.github.io/vega-lite/)
  JSON spec (bar/line/area/scatter/pie). Rendered to static SVG at build time via
  `vl_convert` (no browser, no JS) and auto-themed to Mocha — you rarely need to style
  it. Your own `config`/`background` in the spec overrides the theme. Like diagrams,
  charts are cached by content hash and get click-to-enlarge. A malformed spec fails the
  build. Example:
  ````md
  ```vega
  {
    "data": {"values": [{"m":"Jan","n":12},{"m":"Feb","n":19}]},
    "mark": "bar",
    "encoding": {
      "x": {"field":"m","type":"nominal"},
      "y": {"field":"n","type":"quantitative"}
    }
  }
  ```
  ````
- **Code** — any normally tagged fenced block (` ```go `, etc.).
- **Timeline / dialogue** — hand-authored raw HTML blocks (`<ol class="timeline">`,
  `<div class="dialogue">`); keep a blank line before and after so goldmark treats them
  as HTML blocks. See the sample essay for exact markup.

The sample essay `content/essays/2026-07-11-on-building-a-website-out-of-files.md`
exercises every one of these.

### Hydrating link chips

External `{.margin}` chips need `domain`, `title`, and `desc` attributes. Rather than
writing those by hand, run:

```sh
just hydrate content/essays/my-post.md
```

It fetches each external chip's URL, extracts `<title>` and the meta description, and
writes the attributes back into the file. It's **idempotent** — already-hydrated chips
are skipped, so it's safe to run over the whole directory:

```sh
just hydrate content/essays/*.md
```

If you forget, `just build` fails with the exact `just hydrate …` command to run. This is
the only tool that touches the network.

---

## Theme

The site commits to a single dark palette, **Catppuccin Mocha**. Every color is a CSS
custom property in `assets/css/tokens.css` — change the tokens there to re-theme the
whole site. All text/background pairs pass WCAG AA contrast.

Note on interactive touches (both **zero-JavaScript**, per the site's hard constraint):
the diagram click-to-enlarge modal uses a hidden-checkbox toggle (not `:target`, which
would hijack scroll), and background scroll is locked while it's open via
`html:has(.diagram-toggle:checked)`.

---

## Deploy

Two paths to the same R2 bucket (`gordyclark-com`):

### Token-free — wrangler + OAuth (recommended)

```sh
just login       # one-time browser login; session cached, nothing to store
just deploy-api  # build + upload static/ with correct Content-Type / Cache-Control
```

`deploy-api` runs `scripts/deploy-r2.sh`, uploading each file via
`wrangler r2 object put`. HTML gets a short cache TTL (edits show up fast); the
content-hashed CSS and font are marked immutable for a year. wrangler has no
object-`list`, so this path can't prune stale objects — the only file that changes name
between builds is `style.<hash>.css`, and a leftover old one is harmless (unreferenced).

### S3 keys — rclone (clean-mirror)

```sh
just deploy      # build + rclone sync static/ r2:gordyclark-com
```

`rclone sync` deletes remote objects no longer present locally, so use this when you
want the bucket pruned to exactly match `static/`. One-time setup: create an R2
**Access Key ID + Secret** (R2 → *Manage R2 API Tokens* → Object Read & Write, scoped to
the bucket), then:

```sh
rclone config update r2 \
  access_key_id='YOUR_KEY_ID' \
  secret_access_key='YOUR_SECRET'
rclone ls r2:gordyclark-com    # verify: lists the uploaded files
```

The `r2` remote already has the correct endpoint
(`https://<account-id>.r2.cloudflarestorage.com`), provider, and region; only the keys
need filling in. Credentials live in `~/.config/rclone/rclone.conf`, never committed.

### Serving (custom domain + index resolution)

The bucket is served at `gordyclark.com` via a custom domain
(R2 → bucket → Settings → Public access → Custom Domains).

R2 custom domains **don't** resolve directory indexes, so a Cloudflare
**Transform Rule → Rewrite URL** appends `index.html` to any path ending in `/`:

```
If:    URI Path  ends with  /          (operator MUST be "ends with", not "equals",
Then:  Rewrite Path (Dynamic) to:       or nested paths like /essays/foo/ will 404)
       concat(http.request.uri.path, "index.html")
```

This makes `/`, `/essays/<slug>/`, and `/tags/<tag>/` resolve to their `index.html`.

---

## Notes & known gaps

- **`content/pages/` isn't rendered.** `about.md`/`colophon.md` exist as source but the
  pipeline only emits essays, the index, and tag pages. The homepage carries a short
  intro instead of a standalone About page. Wiring up page rendering is a small
  follow-up if wanted.
- **`static/` and `.cache/` are gitignored** — the build output is never committed.
- **`:has()` support**: the scroll-lock and a couple of CSS niceties use `:has()`,
  supported in current evergreen browsers.
