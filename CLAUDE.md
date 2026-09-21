# CLAUDE.md — gordyclark.com

Guidance for agents working in this repo.

## Always work inside the nix dev shell

`go`, `d2` and `just` come from the flake, not the host machine. Prefix
commands, or enter the shell first:

```sh
nix develop --command bash -c 'just build'
nix develop --command bash -c 'go test ./...'
```

Running the host's `go` gives a different toolchain version and no `d2`, so
diagram rendering fails.

| Task | Command |
|---|---|
| Build the site into `./static` | `just build` |
| Build + serve at :8000 | `just preview` |
| Run tests | `just test` |
| Hydrate link chips | `just hydrate <file.md>` |
| Deploy to R2 | `just deploy-api` |

## Choosing a content type

**This is the first decision for any new post.** The directory determines the
content type, the page template and the published URL — there is no `type:`
field in frontmatter.

| Asked for | Put the file in | Published at | Template |
|---|---|---|---|
| A **list** article or post ("top N", "N things", any enumerated post) | `content/lists/` | `/lists/<slug>/` | `templates/list.html.tmpl` |
| A **text** post, blog post, or short write-up | `content/blog/` | `/blog/<slug>/` | `templates/text.html.tmpl` |
| An **essay** (long-form, uses margin notes and citations) | `content/essays/` | `/essays/<slug>/` | `templates/essay.html.tmpl` |

Do not add a new post to `content/essays/` by default. Essays are the
long-form format with marginalia; a list or a short post belongs in its own
directory.

Filenames are conventionally `YYYY-MM-DD-<slug>.md`, but only the `slug`
frontmatter field determines the URL.

## Frontmatter

Every content file starts with a YAML block. Required: `title`, `slug`,
`date`. Everything else has a default.

```yaml
---
title: "Five Things Worth Keeping"
subtitle: "Habits and tools that survived a decade of churn."
slug: five-things-worth-keeping
date: 2026-09-21
author: Gordy Clark          # optional; defaults to "Gordy Clark"
tags: [lists, engineering]
status: finished             # `finished` or `draft` — ONLY these two values
hero: /img/hero.jpg          # optional image above the article
hero_alt: "Describe it."     # REQUIRED whenever `hero` is set
reading_time_override: null  # null = compute from word count
---
```

Notes:

- `status` must be exactly `finished` or `draft`. Any other value (e.g.
  `active`) is treated as not-finished, so the post is built at its URL but
  **excluded from the homepage, section indexes and tag pages**.
- Slugs are unique across *all* content types. A collision fails the build.
- Setting `hero` without `hero_alt` fails the build, so a hero image can never
  ship without alt text.

## Images: the ```img block

Use a fenced `img` block wherever an image belongs. It works in every content
type, including inside a list item.

````md
```img
src="/img/photo.jpg"
align="left"
alt="Required — describe the image"
caption="Optional caption"
```
````

- `src` and `alt` are **required**; a missing `alt` fails the build.
- `align` is `left`, `right` or `center` (default `center`). Any other value
  fails the build.
- `left`/`right` float the image and let text wrap around it on wide screens,
  and become full-width below the 620px breakpoint.
- Images live in `content/img/` and are copied to the output tree
  automatically, so `content/img/photo.jpg` is referenced as `/img/photo.jpg`.

## Tables

Write an ordinary GFM table; the renderer does the rest. Every table is wrapped
in a `.table-scroll` div and every body cell gets a `data-label` carrying its
column's header (`labelTableCells` in `internal/render/essay.go`).

Both exist because a table is the only block whose minimum width can exceed the
column it sits in — its minimum is the sum of its columns' longest words. Left
alone in a grid cell, that minimum widens the grid track, the track widens the
page, and a phone renders the whole article scaled down to fit the table. The
wrapper catches the overflow; below 620px the rows stop being a table
altogether and stack into labelled cards, which is what the `data-label` is
for.

Keep the scrolling on the wrapper, not on the `<table>`: `display: block` on a
table stops it being a table box, and a narrow table then shrink-wraps its
content instead of filling the column.

## The post metadata box

Every content kind renders a `.post-meta` box (breadcrumb, author, date,
reading time, tags). It is built in Go (`internal/render/margincards.go`) and
placed by the page template inside `.article-header`, which is itself a
two-column grid whose columns match `.article-grid`. The box therefore sits in
the right-hand rail level with the title, heading the marginalia column the
article goes on to use.

**Do not move it into `.article-grid`.** Every top-level markdown block becomes
its own grid row, and with `align-items: start` a row is as tall as its tallest
cell — so a box in the first row's margin cell stretches that row and opens a
gap between a leading heading and the text under it. Absolute positioning is
not a workaround either; it was tried and escaped its container.
`TestPostMetaIsNotInsideTheArticleGrid` guards this.

`.article-header` and `.article-grid` must declare the same
`grid-template-columns` and `column-gap`, or the box stops lining up with the
rail. `TestHeaderRailMatchesArticleGridColumns` guards that.

Templates must not add a separate byline or tag list; the box is the single
place a post states its metadata.

## Tag colors

Tags are colored automatically. `tagColorClass` in
`internal/render/tagcolor.go` hashes the tag name (FNV-1a) to one of the
`--tag-1` .. `--tag-8` hues in `tokens.css`, exposed as `.tag-c1` .. `.tag-c8`
in `components/tag.css`. Templates call `{{tagColor .}}`.

The hash means a tag gets a color the first time it is used, with nobody
assigning one, and keeps the same color on every page and across rebuilds. Do
not replace it with a random draw — that would repaint every tag on every
build. Changing `tagPaletteSize` re-colors existing tags, since the index is
taken modulo that number.

## List posts

A list post's items are ordinary level-2 headings:

```md
## First Thing

Body text, images, or both.

## Second Thing

Body text, images, or both.
```

A numbered table of contents is generated automatically from those headings
and placed at the top of the page; each heading gets an anchor id. Do not
hand-write a TOC.

## Deployment: Cloudflare Pages

The site is moving to **Cloudflare Pages**, which watches `main` and builds on
every push — no API token, no GitHub Actions workflow, no deploy step to run.
Pages runs `go run ./cmd/render` with `GO_VERSION=1.26.4` and serves the
`static/` output. See the README's Deploy section for the one-time dashboard
setup.

Until that connection is made, the live site is served from **R2** and only
updates when someone runs `just deploy-api` locally. Pushing to `main` alone
does not publish anything yet.

**The cache is load-bearing — this is the rule most likely to break a deploy.**
The Pages build image has Go but **no `d2` and no `python3`/`vl_convert`**.
Both renderers read their content-hashed cache before invoking the external
tool, and `.cache/` is committed so a clean checkout builds with Go alone.

Therefore: **if you change a ` ```d2 ` or ` ```vega ` block, you must run
`just build` and commit the regenerated `.cache/` files along with the
change.** Committing the source edit alone produces a cache miss on the remote
build and fails it. Ordinary prose, frontmatter, tables and ` ```img ` edits do
not touch the cache and are safe to push from anywhere.

The build also emits three files Cloudflare parses as configuration rather than
serving: `404.html` (without one, a site is treated as a single-page app and `/`
is served for every unmatched path), `_headers` (the default is
`max-age=0, must-revalidate`), and `_redirects`. All three are generated — edit
them in `internal/render/render.go`, never in `static/`.

## Hard rules

- **Never move the reader's scroll position.** No JavaScript scroll
  manipulation, and no `:target` CSS that shifts layout. Interactive pieces use
  the hidden-checkbox toggle pattern (see `assets/css/components/figure.css`).
  Plain `<a href="#id">` anchors are fine — following one is the reader's own
  click.
- **No colors outside `assets/css/tokens.css`.** Component CSS consumes
  `var(--ink)`, `var(--accent)`, `var(--rule)` and friends. Never hardcode a
  hex value in a component file; changing `tokens.css` must re-theme the whole
  site. The one exception is `print.css`, which is deliberately plain black on
  white and overrides the accents so headings stay legible on paper.
- **Heading colors follow a two-tone hierarchy** set in `type.css`: `h2` uses
  `--accent` (blue), `h3`-`h6` use `--accent-2` (peach). The page title (`h1`)
  and all body text stay `--ink`. Section dividers and `<hr>` use
  `--rule-accent`; structural borders (the margin rail, tag pills) stay the
  neutral `--rule`.
- **New CSS must be added to `assets/css/manifest.txt`** or it is silently
  never bundled. Order matters; `tokens.css` stays first.
- **Grid tracks holding article content are `minmax(0, 1fr)`, never a bare
  `1fr`** — including in the mobile overrides, where the two-column grid
  collapses to one. `1fr` means `minmax(auto, 1fr)`, and that `auto` minimum is
  the item's min-content size, so one wide child stretches the track, the grid
  and the page. `TestArticleGridTracksNeverUseBareFr` guards this.
- **URLs derive from `content.Kind`.** Never hardcode `/essays/`, `/blog/` or
  `/lists/` in a template or Go file — use `Kind.URLPrefix()`, `Kind.URL(slug)`
  or an `IndexEntry`'s `.URL()`.
- Run `just test` before committing.
- **Changing a ` ```d2 ` or ` ```vega ` block means rebuilding and committing
  `.cache/`** — see Deployment above.

## Architecture

```
content/{essays,blog,lists}/*.md   source posts
  -> internal/content   frontmatter parsing, Kind, merged site index
  -> internal/render    goldmark pipeline, margin column, page writing
  -> internal/postimage ```img blocks
  -> internal/listtoc   list-post table of contents
  -> internal/margin    {...} link attributes, margin item model
  -> static/            generated site (not hand-edited)
```

`internal/render/essay.go` renders one document of any kind: it walks
top-level blocks, pairs each with a margin cell, and dispatches fenced blocks
(`d2`, `vega`, `img`, or a language for syntax highlighting) in
`renderBlockContent`.
