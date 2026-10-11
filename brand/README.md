# Grove identity

The approved identity is the **original Grove C Git companion**. Preserve the three rounded-square crowns, two oval eyes, low root node, straight left branch and bent right branch. The later material and colour studies were rejected.

`sources/grove-c-approved.png` is the untouched approved master. The manifest records its SHA-256. Full-colour app icons, favicons and the site's primary identity are resized from this image. The neutral white background is intentional; no satin, glass, metal or alternate palette is selected.

## Choose the right file

| Surface | Files |
| --- | --- |
| Original colour artwork | `logos/grove-original.png`, `.webp`, `.jpg`, `.tiff`, `.svg` |
| Transparent colour companion | `logos/grove-transparent.png`, `.webp`; `logos/grove-color.svg` |
| Monochrome and inverse | `logos/grove-monochrome.svg`, `logos/grove-white.svg` |
| Wordmark | `logos/grove-wordmark-light.*`, `logos/grove-wordmark-dark.*` |
| Print / layout | `logos/grove-color.pdf`, `logos/grove-monochrome.pdf`, `logos/grove-wordmark-light.pdf` |
| macOS | `app-icons/macos/Grove.icns`, `Grove.iconset/`, `Assets.xcassets/` |
| iOS / iPadOS | `app-icons/ios/Assets.xcassets/` with phone, tablet and 1024px marketing images |
| Windows | `app-icons/windows/Grove.ico` (16, 24, 32, 48, 64, 128 and 256px) |
| Linux | `app-icons/linux/hicolor/` |
| Android | `app-icons/android/res/`: legacy density assets and adaptive foreground/background resources |
| Browser and saved-site icons | `web/`: multi-size ICO, PNG, Apple touch icon, maskable variants and Safari mask SVG |
| Menu bar / tray | `menu-bar/GroveTemplate.svg`, `.pdf`, `GroveTemplate.imageset/`, black/white PNG at 16–32pt and 1x/2x/3x |
| Social | `social/`: avatars and 1200×630 share cards |
| Visual review | `preview/index.html` and `preview/asset-review.png` |

The colour SVGs embed the approved raster artwork, **not editable vector paths**. This preserves the chosen gradients and geometry without substituting a redraw. The wordmark lettering is outlined; its colour emblem is raster-backed. The monochrome SVGs and PDFs contain actual vector paths. Upscaling a colour SVG does not create additional detail beyond its 1254px source.

The transparent and monochrome companions are derived utility artwork. The original PNG remains authoritative. Use the original white tile or inverse monochrome on dark surfaces: the original dark-green branches do not have enough contrast directly on a dark canvas. The template glyph has small negative-space channels to keep its Git loops distinguishable in one colour. The illustration source in `sources/grove-monochrome.png` has a generated matte; the exporter traces only its black ink, so that matte is absent from all delivered template assets.

## Platform usage

- Desktop PNGs and ICNS include an inset rounded white tile and transparent outer corners. Do not round or pad them again.
- iOS, web and Android legacy canvases are square and unmasked. Let the receiving platform mask them.
- Android adaptive icons use an isolated foreground and a white background. Their content is kept within the central safe area; copy the supplied resources together.
- `GroveTemplate.imageset` declares template rendering. For AppKit images loaded directly, set `isTemplate = true`; let the system choose the foreground colour. The white PNG/SVG files are for consumers without template rendering.
- These are prepared assets. Grove currently ships as a CLI and website, so no native app has been invented, signed, installed or released as part of this work.
- The website manifest uses `display: browser`. Brand installation does not add an offline app or service worker.

## Rebuild and verify

From the repository root:

```sh
pnpm --dir site install --frozen-lockfile
pnpm --dir site brand:build
pnpm --dir site brand:check
pnpm --dir site build
```

The generator needs Node 20+, the site's dev dependencies, and macOS `iconutil` for ICNS packaging. It only reads the repo-local sources. It writes the export pack, copies the assets used by the website into `site/public/brand/` and the App Router metadata files, and records their checksums in `site/brand-assets.json`.

Every normal `pnpm --dir site build` verifies these deployment files before Next.js runs. A missing, edited or stale icon or ZIP fails the build with a repair command. This deployment check uses only Node built-ins and files inside `site/`, so a site-only Linux/Vercel build does not need macOS tools, the root asset pack, or the iCloud workspace. Full native asset regeneration remains an explicit command.

The root Makefile exposes the same workflow as `make brand`, `make brand-check`, and `make site-build`.

The site's navigation, closing call to action, favicon, Apple touch icon, manifest and Open Graph image use this identity. `../README.md` uses the desktop icon. No CLI output or captured terminal examples were changed.

`manifest.json` inventories generated files, dimensions, alpha channels and checksums. The downloadable ZIP contains this guide, manifest and the reusable exports. There is no separate trademark-clearance or asset-licensing assertion implied by this pack.
