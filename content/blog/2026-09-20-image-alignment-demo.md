---
title: "Image Alignment Demo"
subtitle: "Exercises every alignment option of the img block."
slug: image-alignment-demo
date: 2026-09-20
tags: [blog]
status: draft
hero: /img/sample.png
hero_alt: "A placeholder hero image."
---

A left-aligned image floats and lets the text wrap around it.

```img
src="/img/sample.png"
align="left"
alt="A left-aligned placeholder"
caption="Floated left, text wraps."
```

This paragraph should wrap around the left-floated image above it, filling the
space beside it rather than starting below. On a narrow screen the float is
dropped and the image becomes full width instead.

```img
src="/img/sample.png"
align="right"
alt="A right-aligned placeholder"
```

This paragraph sits beside the right-floated image. That one has no caption, so
no figcaption element should be emitted for it at all.

```img
src="/img/sample.png"
align="center"
alt="A centered placeholder"
caption="Centered, the default."
```

A centered image is the default when align is omitted.
