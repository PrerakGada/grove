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
- `src/components/InstallButton.tsx` — the "Install with Go" button; see below

## The install button and the counter

The page reports to the shared product counter at `api.prerakgada.in` (slug
`grove`), wired per `~/work/company/infra/product-analytics.md`: the tracker
`<script>` sits in `src/app/layout.tsx` with `data-form`, `data-command-label="Go"`
and `data-command={INSTALL}`. Cookieless, so no consent banner; the footer says
what it counts and what the form asks (step 5 of that guide).

**The `go install` command is never printed on the page.** Both Install buttons
carry `data-pt-command`; the tracker opens its optional form (name, email, role,
referral, every field optional) and shows the command after it, submitted
or skipped. Three fallbacks keep an install from depending on the counter:
`<noscript>` copies for JavaScript-off visitors; `InstallButton` revealing the
command in place when the tracker never loaded (ad blockers do block
`tracker.js`) or did not take the click; and the tracker's own fallback when its
API is slow. Do not add the command back as visible text, and keep `INSTALL` in
`src/lib/content.ts` the single source for all of these.

There is no CSP on this site. If one is ever added, it must allow
`https://api.prerakgada.in` in `script-src`, `connect-src` and `style-src`.

## Commands

```bash
pnpm dev                 # localhost:3000
pnpm build               # must pass before deploying
vercel deploy --prod     # scope: engaze, project: grove
```

`vercel deploy` uploads the working tree, not the last commit. When the tree holds
uncommitted work that is not meant to ship, deploy an export of `HEAD` instead:

```bash
D=$(mktemp -d) && git -C .. archive HEAD site | tar -x -C "$D" \
  && mkdir -p "$D/site/.vercel" && cp .vercel/project.json "$D/site/.vercel/" \
  && (cd "$D/site" && vercel deploy --prod --scope engaze --yes)
```

## Deployments

- **10 Oct 2026**: `3ad0d2b` to production (the counter and install gate from
  `a3b5cf6`, tracker `?v=4`), deployed from an export of `HEAD` with the command
  above, not from the working tree, so the uncommitted 12 Sep brand round did
  not ship. Deployment `dpl_8cYpJtSiBCTP6ZPDpvp32mU8qBm3`
  (`grove-87w60u1qh-engaze.vercel.app`), aliased to grove.prerakgada.in.
  Checked with curl on both: one `tracker.js?v=4` tag, two `data-pt-command`
  buttons, the command only in `<noscript>` and `data-command`, the Google /
  Apple line in the footer. Replaced the 5 Aug deployment of `7a93cf4`.
  The tag is `?v=4` because Cloudflare cached the old tracker under `?v=3`;
  never point the page at a version number fetched before the backend shipped it.
- **10 Oct 2026, later**: `4fba654` to production. Prerak moved Google / Apple
  sign-in off landing-page download forms (it belongs inside apps with a login);
  the shared backend now serves the form with both providers off. The footer no
  longer says the form can be filled by signing in with Google or Apple, and the
  tag is `?v=5` so the page stops loading the Cloudflare-cached `?v=4` copy that
  still offers Continue with Apple. Deployed from an export of the commit with the
  command above. Deployment `dpl_AZTWXVdbgY6yCJreDDwv9ZuxPALa`
  (`grove-9vbua46r4-engaze.vercel.app`), aliased to grove.prerakgada.in. Checked
  with no-cache curl on both: `tracker.js?v=5` only (the tag and its RSC copy),
  two `data-pt-command` buttons, the command only in `<noscript>`, `data-command`
  and script payloads, no sign-in wording (the remaining Google / Apple hits are
  the `googlebot` meta and the "Apple Color Emoji" font stack). Still no CSP.
