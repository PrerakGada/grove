<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

# site/ — the Grove landing page

Next.js 16 (App Router, Turbopack) + Tailwind v4 + TypeScript, deployed to
Vercel. Independent of the Go build.

Breaking changes that bit during the build: Turbopack is the default bundler,
`params`/`id` reaching image-generation files are Promises, and Next no longer
overrides `scroll-behavior` unless `<html data-scroll-behavior="smooth">` is
set. Start from `node_modules/next/dist/docs/01-app/02-guides/upgrading/version-16.md`.

## The two rules that matter

**Terminal blocks are captured, never written.** Everything under `term` in
`src/lib/content.ts` is literal stdout from a real `grove` binary run against a
scratch monorepo — only the checkout path was rewritten to `~/code/acme`. To
change one, re-run the command and paste the result. Inventing a
plausible-looking line is the one thing that breaks this page, because its whole
argument is that the numbers are real.

**The palette is the CLI's palette.** `internal/ui/ui.go` styles output with
ANSI 1–6 — green `✓`, blue `→`, yellow `!`, red `✗`, cyan for keys. Those five
roles are the site's entire colour vocabulary, declared as tokens in
`src/app/globals.css` with their contrast ratios against the canvas. A sixth
colour here means adding it to the CLI first.

Mono needs `font-feature-settings: "liga" 0, "calt" 0` — JetBrains Mono ligates
`--` and `<=`, which silently corrupts captured output.

## Layout

- `src/lib/content.ts` — all copy, terminal captures, failure stories, commands
- `src/components/BandRuler.tsx` — the hero diagram. Its mode toggle re-runs the
  predecessor's `band + slot*100` so the central claim can be falsified on the
  page rather than merely asserted.
- `src/components/Terminal.tsx` — colours a line by its leading glyph, the same
  rule the CLI uses
- `src/app/opengraph-image.tsx` — needs the TTFs in `assets/`; Satori cannot use
  the woff2 that `next/font` downloads

## Commands

```bash
pnpm dev                 # localhost:3000
pnpm build               # must pass before deploying
vercel deploy --prod     # scope: engaze, project: grove
```
