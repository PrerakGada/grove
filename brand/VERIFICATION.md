# Brand integration verification

Verified locally on 12 September 2026.

- Approved source preserved byte-for-byte: `ff13f9af8ff3244d4027bf3a41c1522f1bfda5c2204625a5736e24a28e8b2f1c`.
- `pnpm --dir site brand:check`: 172 exported files, dimensions and checksums verified; ICO and ICNS directory structures valid; all Apple catalog entries resolve at their declared dimensions; iOS images opaque; desktop corners transparent; menu-bar artwork black-only with alpha; website copies match.
- `xcrun actool` compiled both the macOS and iPhone/iPad asset catalogs into `Assets.car`. This is asset compilation, not a signed native app build.
- `pnpm --dir site build`: success, including static favicon, Apple icon, manifest and Open Graph routes.
- `pnpm --dir site lint`: no issues.
- Browser inspection at 1440px and 390px: new mark renders in navigation and closing call to action; no horizontal page overflow; mobile footer includes the brand ZIP download.
- HTTP checks: favicon ICO, SVG icon, Apple touch icon, Safari mask, manifest, four manifest icon files, Open Graph PNG and downloadable ZIP all return 200 with the expected image/manifest/archive MIME types.
- Open Graph and Twitter metadata point to the generated social image. Local metadata uses the existing localhost:4310 fallback; production origin still comes from the existing deployment environment configuration.
- All four PDF exports rendered and inspected. The initial SVG viewport scaling issue was fixed; the complete monochrome logo and 22pt template now fit their PDF pages.
- Native-size favicon and template preview, desktop/mobile site captures and the social preview were visually inspected. `preview/asset-review.png` was opened in VS Code.
- ZIP archive integrity checked with `unzip -tq`.

No production deployment, commit or push performed. Native app installation and platform runtime behaviour remain outside this CLI/site branding integration.

## Build integration follow-up

All reusable media remains inside `brand/`; every website deployment copy is recorded in `site/brand-assets.json`. Normal site builds now verify that inventory before Next.js runs. The verification script uses only Node built-ins and the deployed `site/` files, keeping site-only deployment independent of the repository's native export tools.
