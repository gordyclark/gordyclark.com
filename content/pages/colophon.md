---
title: "Colophon"
slug: colophon
date: 2026-07-11
---

This site is built the way I wish more software were built: out of plain files, by a small program, with as few moving parts as I could get away with.

Every page begins life as a markdown file on disk. A short Go program reads those files and turns them into HTML, using [goldmark](https://github.com/yuin/goldmark) to do the parsing. Goldmark is strict, standards-compliant, and easy to extend, which is exactly what I want from the one dependency that touches every word I publish.

Code blocks are highlighted at build time by Chroma, so the syntax colors are baked directly into the HTML — there is no client-side highlighter waking up to re-do work the build already finished. Diagrams are written as D2 source in the markdown itself and rendered to inline SVG during the build, which means a diagram and the sentence explaining it live in the same file and can never drift apart.

The body text is set in Public Sans, self-hosted rather than pulled from a font CDN, so the site depends on no third party to render correctly and phones home to nobody.

There is no JavaScript. Not a little — none. Every page is a static document that renders in any browser without a runtime, a framework, or a hydration step.

The finished output is a folder of static files, deployed to Cloudflare R2. The whole thing can be rebuilt from scratch, copied wholesale, or handed to someone else without a manual. That portability is the point.
