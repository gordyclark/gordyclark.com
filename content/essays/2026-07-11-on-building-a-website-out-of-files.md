---
title: "On Building a Website Out of Files"
subtitle: "Why a markdown-to-HTML pipeline with no server and no JavaScript is still the right call in 2026."
slug: on-building-a-website-out-of-files
date: 2026-07-11
tags: [essays, static-web]
status: finished
reading_time_override: null
---

Every website I have ever regretted maintaining had the same defect at its center: the content lived somewhere I could not see it. It sat in a database, or behind an admin panel, or inside a headless CMS whose export format was theoretically documented and practically a hostage negotiation. When I wanted to move, I could not just copy the words. I had to migrate the words. Those are different verbs, and the difference between them is roughly a weekend.

So this site is made of files. Each essay is a markdown file on disk, with a little frontmatter at the top and prose below it. There is no database, no server holding my writing captive, no runtime that has to be alive for the page to exist. When I want to read what I wrote, I open the file. When I want to back it up, I copy the folder. When I want to leave, I take the folder with me, and nothing is lost, because the folder was always the whole thing.

That single decision — files are the source of truth — cascades into every other choice worth making. If the content is just files, the build step is just a function: markdown goes in, HTML comes out, and the same input always produces the same output. That determinism is not a luxury. It is the property that lets me trust a diff. Static site generators converge on a few good shapes.[^cite:gwern-design] I did not invent this pattern; I just stopped fighting it.

The generator is a small Go program that leans on [goldmark](https://github.com/yuin/goldmark){.margin domain="github.com" title="yuin/goldmark" desc="A markdown parser written in Go — easy to extend, standards compliant, well structured."} to do the parsing, so I am not the one maintaining a markdown grammar at two in the morning. Goldmark is strict where it should be and extensible where I need it, which means the weird conventions this site relies on — margin chips, citation footnotes, diagrams — are extensions I bolt on, not forks I babysit.

The shape of the pipeline is deliberately dull:

```d2
content: {shape: rectangle}
render: {shape: rectangle}
output: {shape: rectangle}
content -> render -> output
```

Content flows into a render step and out the other side as finished HTML. There is no fourth box, and the absence of a fourth box is the point. Diagrams like this one are themselves just text: I write [D2](https://d2lang.com){.margin domain="d2lang.com" title="D2 — Declarative Diagramming" desc="A modern diagram scripting language. Text to diagram."} source in a fenced block, the build renders it, and the diagram lives in the same file as the sentence explaining it. Nothing to click, nothing to load, nothing to keep alive.

The build fails loud. If a footnote points at a definition that does not exist, if a citation key is missing from the bibliography, if an internal link targets an essay that was never written, the build stops and tells me exactly which file and which line offended it. I would rather ship nothing than ship something quietly broken, and a pipeline that swallows its own errors is just a slower way to publish mistakes. Here is the sort of check that runs everywhere in the codebase:

```go
func readingTime(words int) int {
	const wpm = 220
	minutes := words / wpm
	if minutes < 1 {
		return 1
	}
	return minutes
}
```

It is not clever. It does not need to be. A small pure function that always returns the same answer for the same input is the kind of thing I can read once, trust forever, and never think about again — which is exactly the deal I want with my tools.

There is no JavaScript on this site. Not "minimal JavaScript," not "progressive enhancement," none. The pages are documents, and documents render in a browser without a runtime, without a hydration step, without a spinner promising that the content is coming. This is not nostalgia. It is a bet that the plainest thing that could work will still work in ten years, when the frameworks I might have reached for have been deprecated, rewritten, and deprecated again. If you want the longer version of that argument, it is [why I keep the pipeline boring](/essays/boring-technology-at-workmind.md){.margin}, and I will not repeat it here.

The site has grown up slowly, on purpose. The milestones are small and unglamorous, which is how I know they were real:

<ol class="timeline">
  <li><div class="tl-date">March 2024</div><div class="tl-title">First static prototype</div><p class="tl-desc">One Go file, one template, no CSS — just to prove the pipeline end to end.</p></li>
  <li><div class="tl-date">September 2024</div><div class="tl-title">Footnotes and citations</div><p class="tl-desc">Added a bibliography file and taught the build to fail when a citation key was missing.</p></li>
  <li><div class="tl-date">July 2026</div><div class="tl-title">Margin chips and diagrams</div><p class="tl-desc">Inline D2 rendering and hydrated link previews, still zero JavaScript on the page.</p></li>
</ol>

I ran the design past a skeptical reader before I committed to it:

<div class="dialogue">
  <div class="line speaker-a"><span class="speaker">Gordy</span><p>No server, no database, no JavaScript. The whole site is a folder of text files and a build step.</p></div>
  <div class="line speaker-b"><span class="speaker">Claude</span><p>What happens when you want a comment section, or search, or anything dynamic?</p></div>
  <div class="line speaker-a"><span class="speaker">Gordy</span><p>Then I add exactly that, in the smallest way that works, and only once I actually want it. Until then the absence is a feature. I am not paying rent on capabilities I never use.</p></div>
</div>

That is the whole philosophy. Start with files. Add machinery only when it earns its keep.[^complexity-caveat] Make the build shout the moment anything drifts out of true. The result is a website I can read on disk, rebuild from scratch, and hand to someone else without a manual. In 2026, with an entire industry pointed the other way, that still feels like the right call.

[^complexity-caveat]: This isn't a rejection of complexity where it genuinely earns its keep — only of complexity adopted by default.
