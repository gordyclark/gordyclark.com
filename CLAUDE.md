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

The build also emits `404.html` (Pages treats a site without one as an SPA and
serves `/` for every unmatched path) and `_headers` (Pages defaults to
`max-age=0, must-revalidate`). Both are generated — edit them in
`internal/render/render.go`, never in `static/`.

## Hard rules

- **Never move the reader's scroll position.** No JavaScript scroll
  manipulation, and no `:target` CSS that shifts layout. Interactive pieces use
  the hidden-checkbox toggle pattern (see `assets/css/components/figure.css`).
  Plain `<a href="#id">` anchors are fine — following one is the reader's own
  click.
- **No colors outside `assets/css/tokens.css`.** Component CSS consumes
  `var(--ink)`, `var(--accent)`, `var(--rule)` and friends. Never hardcode a
  hex value in a component file; changing `tokens.css` must re-theme the whole
  site.
- **New CSS must be added to `assets/css/manifest.txt`** or it is silently
  never bundled. Order matters; `tokens.css` stays first.
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
