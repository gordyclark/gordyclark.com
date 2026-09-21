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
| A **collection** (a page generated from a CSV, e.g. the books list) | `content/collections/` | `/collections/<slug>/` | `templates/collection.html.tmpl` |

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
data: books.csv              # collections ONLY; names a file in content/data/
---
```

Notes:

- `status` must be exactly `finished` or `draft`. Any other value (e.g.
  `active`) is treated as not-finished, so the post is built at its URL but
  **excluded from the homepage, section indexes and tag pages**.
- Slugs are unique across *all* content types. A collision fails the build.
- Setting `hero` without `hero_alt` fails the build, so a hero image can never
  ship without alt text.
- `data` is required on a collection and forbidden everywhere else; both cases
  fail the build rather than silently rendering the wrong thing.

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
- **A floated image must be written ABOVE the text that wraps it.** A float only
  affects content that follows it. `renderEssay` then holds that image's
  content cell open and writes the following blocks into it, because each
  top-level block normally gets its own grid cell and a float cannot escape the
  block containing it — an image alone in a cell has nothing to wrap. The run
  ends at the next heading, thematic break or fenced block, so sections never
  bleed together and two floats never collide in one cell. `isFloatedImage` and
  `endsFloatRun` in `essay.go` decide this; the `TestFloat*` tests guard it,
  including that the content and margin columns stay paired one-to-one.
- Images live in `content/img/` and are copied to the output tree
  automatically, so `content/img/photo.jpg` is referenced as `/img/photo.jpg`.

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

## The site header

The header scrolls away with the page; it is deliberately not sticky. Its Books
link comes from the `booksURL` template function, which builds the path from
`content.KindCollection.URL()` — the rule that forbids hardcoded section URLs
elsewhere applies in the nav too.

Back-to-top is **not** in this header. It belongs to the collection jump bar,
which is the bar that is actually on screen while a reader is deep in a long
list. See below.

## Collection pages

A collection renders a CSV into several sorted views of the same data. The
books list is the one in use: `content/collections/books.md` carries the prose,
and `data: books.csv` points at `content/data/books.csv`.

The CSV needs a `Title,Author,Genre,Notes` header. A row missing a title or an
author fails the build; a blank genre falls into an "Unfiled" section. The CSV
is the source of truth — re-export it from the sheet and replace the file; no
markdown table is maintained by hand.

`internal/collection` does the grouping and sorting, `internal/render/collection.go`
writes the HTML. Three views are built:

- **Author** — by surname (the CSV stores `Surname, First`), A–Z sections.
- **Title** — ignoring a leading `The`/`A`/`An`, so *The Windup Girl* files under W.
- **Genre** — genres alphabetically, books within each by author surname.

Each view's sticky jump bar ends with a "↑ Top" link, set off at the far end
from the letters. It is a plain `<a href="#top">` pointing at an id on `<body>`,
so following it is the reader's own click and the scroll rule holds — never
replace it with a script. Every view needs its own, since only one bar is on
screen at a time; `TestEveryJumpBarEndsWithBackToTop` guards that.

**All three views ship in the same page and a radio button reveals one.** There
are no iframes, no fetches and no JavaScript, which is what keeps the page
inside the scroll rule — there is nothing that *can* move the reader. The radios
are emitted before the sort control and the views so the CSS sibling combinator
(`#sort-x:checked ~ ...`) can reach them; `TestCollectionRadiosPrecedeControlAndViews`
guards that ordering, because reversing it would hide every view.

The radios are `position: fixed` at the viewport's top-left, the same trick
`figure.css` uses: clicking a label focuses its radio and the browser scrolls it
into view, so a radio already on screen makes that scroll a no-op.

Adding a content kind means adding it to `content.AllKinds` plus the switches in
`types.go`; the render pipeline and the index both iterate that slice, so a kind
cannot be picked up by one loop and missed by another.

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
- **URLs derive from `content.Kind`.** Never hardcode `/essays/`, `/blog/` or
  `/lists/` in a template or Go file — use `Kind.URLPrefix()`, `Kind.URL(slug)`
  or an `IndexEntry`'s `.URL()`.
- Run `just test` before committing.
- **Changing a ` ```d2 ` or ` ```vega ` block means rebuilding and committing
  `.cache/`** — see Deployment above.

## Architecture

```
content/{essays,blog,lists,collections}/*.md   source posts
content/data/*.csv                            collection data
  -> internal/content    frontmatter parsing, Kind, merged site index
  -> internal/render     goldmark pipeline, margin column, page writing
  -> internal/postimage  ```img blocks
  -> internal/listtoc    list-post table of contents
  -> internal/collection CSV parsing, sorted/grouped collection views
  -> internal/margin    {...} link attributes, margin item model
  -> static/            generated site (not hand-edited)
```

`internal/render/essay.go` renders one document of any kind: it walks
top-level blocks, pairs each with a margin cell, and dispatches fenced blocks
(`d2`, `vega`, `img`, or a language for syntax highlighting) in
`renderBlockContent`.
