import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { createWriteStream } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import sharp from 'sharp';
import pngToIco from 'png-to-ico';
import potrace from 'potrace';
import PDFDocument from 'pdfkit';
import SVGtoPDF from 'svg-to-pdfkit';

// Production exports only. The approved colour source is never re-illustrated.
const site = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const root = resolve(site, '..');
const brand = join(root, 'brand');
const source = await readFile(join(brand, 'sources/grove-c-approved.png'));
const transparent = await readFile(join(brand, 'sources/grove-transparent.png'));
const checksum = createHash('sha256').update(source).digest('hex');
const inventory = [];
const siteAssets = [];
const white = '#ffffff';
const ink = '#08090a';

async function save(relative, data) {
  const path = join(brand, relative);
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, data);
  inventory.push(relative);
  return path;
}
async function install(relative, data) {
  const path = join(site, relative);
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, data);
  siteAssets.push({
    path: relative,
    bytes: data.length,
    sha256: createHash('sha256').update(data).digest('hex'),
  });
}
const png = (input, size) => sharp(input).resize(size, size).png().toBuffer();
const uri = (data) => `data:image/png;base64,${data.toString('base64')}`;
const wrap = (data, size = 1254) => `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}"><title>Grove Git companion</title><desc>Approved raster artwork embedded losslessly. This colour SVG is not a path-based vector master.</desc><image width="${size}" height="${size}" xlink:href="${uri(data)}"/></svg>`;

// The template illustration is traced to real vector paths. A fixed low
// threshold selects its black ink only, excluding the generated grey matte.
const monochrome = await new Promise((resolve, reject) => {
  potrace.trace(join(brand, 'sources/grove-monochrome.png'), {
    threshold: 90, color: '#000000', background: 'transparent',
    turdSize: 24, optTolerance: 0.25,
  }, (error, svg) => error ? reject(error) : resolve(svg));
});
const inverse = monochrome.replaceAll('fill="#000000"', 'fill="#ffffff"');
await save('logos/grove-color.svg', wrap(transparent));
await save('logos/grove-original.svg', wrap(source));
await save('logos/grove-monochrome.svg', monochrome);
await save('logos/grove-white.svg', inverse);
await save('logos/grove-original.png', source);
await save('logos/grove-transparent.png', transparent);
for (const size of [32, 64, 128, 256, 512, 1024]) {
  await save(`logos/png/grove-white-background-${size}.png`, await png(source, size));
  await save(`logos/png/grove-transparent-${size}.png`, await png(transparent, size));
}
await save('logos/grove-original.jpg', await sharp(source).jpeg({ quality: 95 }).toBuffer());
await save('logos/grove-original.webp', await sharp(source).webp({ lossless: true }).toBuffer());
await save('logos/grove-transparent.webp', await sharp(transparent).webp({ lossless: true }).toBuffer());
await save('logos/grove-original.tiff', await sharp(source).tiff({ compression: 'lzw' }).toBuffer());

// Neutral white app tile only: the rejected finishes are intentionally absent.
// iOS/web/Android receive unmasked square canvases; desktop files have alpha
// outside an inset tile because desktop shells do not supply the same mask.
async function desktop(size) {
  const edge = Math.round(size * 0.81);
  const offset = Math.floor((size - edge) / 2);
  const image = await png(source, edge);
  const radius = Math.round(edge * 0.225);
  const mask = Buffer.from(`<svg width="${edge}" height="${edge}"><rect width="${edge}" height="${edge}" rx="${radius}" fill="white"/></svg>`);
  const tile = await sharp(image).ensureAlpha().composite([{ input: mask, blend: 'dest-in' }]).png().toBuffer();
  return sharp({ create: { width: size, height: size, channels: 4, background: '#00000000' } })
    .composite([{ input: tile, left: offset, top: offset }]).png().toBuffer();
}
for (const size of [16, 24, 32, 48, 64, 128, 256, 512, 1024]) {
  await save(`app-icons/png/grove-${size}.png`, await png(source, size));
  await save(`app-icons/desktop/grove-${size}.png`, await desktop(size));
}
const iconset = join(brand, 'app-icons/macos/Grove.iconset');
for (const size of [16, 32, 128, 256, 512]) {
  await save(`app-icons/macos/Grove.iconset/icon_${size}x${size}.png`, await desktop(size));
  await save(`app-icons/macos/Grove.iconset/icon_${size}x${size}@2x.png`, await desktop(size * 2));
}
execFileSync('iconutil', ['-c', 'icns', iconset, '-o', join(brand, 'app-icons/macos/Grove.icns')]);
inventory.push('app-icons/macos/Grove.icns');

const macImages = [];
for (const size of [16, 32, 128, 256, 512]) for (const scale of [1, 2]) {
  const filename = `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`;
  await save(`app-icons/macos/Assets.xcassets/AppIcon.appiconset/${filename}`, await desktop(size * scale));
  macImages.push({ idiom: 'mac', size: `${size}x${size}`, scale: `${scale}x`, filename });
}
const info = { version: 1, author: 'xcode' };
await save('app-icons/macos/Assets.xcassets/Contents.json', JSON.stringify({ info }, null, 2));
await save('app-icons/macos/Assets.xcassets/AppIcon.appiconset/Contents.json', JSON.stringify({ images: macImages, info }, null, 2));
const iosImages = [];
for (const [idiom, size, scales] of [
  ['iphone', 20, [2, 3]], ['iphone', 29, [2, 3]], ['iphone', 40, [2, 3]], ['iphone', 60, [2, 3]],
  ['ipad', 20, [1, 2]], ['ipad', 29, [1, 2]], ['ipad', 40, [1, 2]], ['ipad', 76, [1, 2]],
  ['ipad', 83.5, [2]], ['ios-marketing', 1024, [1]],
]) for (const scale of scales) {
  const filename = `icon-${idiom}-${size}@${scale}x.png`;
  await save(`app-icons/ios/Assets.xcassets/AppIcon.appiconset/${filename}`, await png(source, size * scale));
  iosImages.push({ idiom, size: `${size}x${size}`, scale: `${scale}x`, filename });
}
await save('app-icons/ios/Assets.xcassets/Contents.json', JSON.stringify({ info }, null, 2));
await save('app-icons/ios/Assets.xcassets/AppIcon.appiconset/Contents.json', JSON.stringify({ images: iosImages, info }, null, 2));

const icoSizes = [16, 24, 32, 48, 64, 128, 256];
await save('app-icons/windows/Grove.ico', await pngToIco(await Promise.all(icoSizes.map(desktop))));
for (const size of [16, 24, 32, 48, 64, 128, 256, 512]) {
  await save(`app-icons/linux/hicolor/${size}x${size}/apps/grove.png`, await desktop(size));
}
await save('app-icons/linux/hicolor/scalable/apps/grove.svg', wrap(source));

for (const [density, size, adaptive] of [['mdpi', 48, 108], ['hdpi', 72, 162], ['xhdpi', 96, 216], ['xxhdpi', 144, 324], ['xxxhdpi', 192, 432]]) {
  await save(`app-icons/android/res/mipmap-${density}/ic_launcher.png`, await png(source, size));
  // The whole mark remains inside Android's central safe zone.
  const foreground = await png(transparent, Math.round(adaptive * 0.63));
  const pad = Math.floor((adaptive - Math.round(adaptive * 0.63)) / 2);
  const layer = await sharp({ create: { width: adaptive, height: adaptive, channels: 4, background: '#00000000' } })
    .composite([{ input: foreground, left: pad, top: pad }]).png().toBuffer();
  await save(`app-icons/android/res/mipmap-${density}/ic_launcher_foreground.png`, layer);
}
await save('app-icons/android/res/values/colors.xml', '<resources><color name="grove_icon_background">#FFFFFF</color></resources>\n');
await save('app-icons/android/res/mipmap-anydpi-v26/ic_launcher.xml', '<?xml version="1.0" encoding="utf-8"?>\n<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android"><background android:drawable="@color/grove_icon_background"/><foreground android:drawable="@mipmap/ic_launcher_foreground"/></adaptive-icon>\n');

const favicon = await pngToIco(await Promise.all([16, 32, 48, 64].map((n) => png(source, n))));
await save('web/favicon.ico', favicon);
await install('src/app/favicon.ico', favicon);
await install('src/app/icon.svg', wrap(await png(source, 256), 256));
const apple = await png(source, 180);
await save('web/apple-touch-icon.png', apple);
await install('src/app/apple-icon.png', apple);
for (const size of [16, 32, 48, 96, 192, 512]) {
  const data = await png(source, size);
  await save(`web/icon-${size}.png`, data);
  await install(`public/brand/icon-${size}.png`, data);
}
for (const size of [192, 512]) {
  const inset = Math.round(size * 0.8);
  const pad = Math.floor((size - inset) / 2);
  const data = await sharp({ create: { width: size, height: size, channels: 3, background: white } })
    .composite([{ input: await png(source, inset), left: pad, top: pad }]).png().toBuffer();
  await save(`web/maskable-${size}.png`, data);
  await install(`public/brand/maskable-${size}.png`, data);
}
await install('public/brand/grove-icon.png', await png(source, 256));
await install('public/brand/grove-original.png', await png(source, 1024));
await install('public/brand/grove-transparent.png', await png(transparent, 512));
await install('public/brand/grove-monochrome.svg', monochrome);
await install('public/brand/grove-white.svg', inverse);
await save('web/safari-pinned-tab.svg', monochrome);
await install('public/brand/safari-pinned-tab.svg', monochrome);

for (const size of [16, 18, 20, 22, 24, 32]) for (const scale of [1, 2, 3]) {
  for (const [name, svg] of [['Template', monochrome], ['White', inverse]]) {
    await save(`menu-bar/png/Grove${name}-${size}@${scale}x.png`, await png(Buffer.from(svg), size * scale));
  }
}
await save('menu-bar/GroveTemplate.svg', monochrome);
await save('menu-bar/GroveWhite.svg', inverse);
const templateImages = [];
for (const scale of [1, 2, 3]) {
  const filename = `GroveTemplate${scale === 1 ? '' : `@${scale}x`}.png`;
  await save(`menu-bar/GroveTemplate.imageset/${filename}`, await png(Buffer.from(monochrome), 22 * scale));
  templateImages.push({ idiom: 'universal', scale: `${scale}x`, filename });
}
await save('menu-bar/GroveTemplate.imageset/Contents.json', JSON.stringify({ images: templateImages, info, properties: { 'template-rendering-intent': 'template' } }, null, 2));

// SVG wordmarks keep the approved image and use outlined existing site type.
const fontkit = await import('fontkit');
const font = fontkit.openSync(join(site, 'assets/JetBrainsMono-Bold.ttf'));
function wordmark(fg, bg) {
  const run = font.layout('grove');
  const scale = 96 / font.unitsPerEm;
  let x = 235;
  const paths = run.glyphs.map((glyph, i) => {
    const position = run.positions[i];
    const p = `<path d="${glyph.path.toSVG()}" transform="translate(${x + position.xOffset * scale},${143 - position.yOffset * scale}) scale(${scale},${-scale})" fill="${fg}"/>`;
    x += position.xAdvance * scale;
    return p;
  }).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="600" height="210" viewBox="0 0 600 210"><title>Grove</title>${bg ? `<rect width="600" height="210" fill="${bg}"/>` : ''}<image x="0" y="0" width="210" height="210" xlink:href="${uri(source)}"/>${paths}</svg>`;
}
for (const [name, fg, bg] of [['light', ink, white], ['dark', '#ffffff', ink]]) {
  const svg = wordmark(fg, bg);
  await save(`logos/grove-wordmark-${name}.svg`, svg);
  await save(`logos/grove-wordmark-${name}.png`, await sharp(Buffer.from(svg)).png().toBuffer());
  await save(`logos/grove-wordmark-${name}.webp`, await sharp(Buffer.from(svg)).webp({ lossless: true }).toBuffer());
}

async function pdf(name, width, height, draw) {
  const path = join(brand, name);
  await mkdir(dirname(path), { recursive: true });
  const doc = new PDFDocument({ size: [width, height], margin: 0, info: { Title: 'Grove approved identity', Author: 'Prerak Gada', CreationDate: new Date('2026-09-12T00:00:00Z'), ModDate: new Date('2026-09-12T00:00:00Z') } });
  const stream = createWriteStream(path);
  doc.pipe(stream);
  draw(doc);
  doc.end();
  await new Promise((resolve, reject) => { stream.on('finish', resolve); stream.on('error', reject); });
  inventory.push(name);
}
function drawSvg(doc, svg, width, height, sourceWidth = 1254, sourceHeight = 1254) {
  // SVG-to-PDFKit honours intrinsic SVG dimensions ahead of viewport options.
  // Scale the document explicitly, in points, so tiny template PDFs are not clipped.
  doc.save().scale(width / sourceWidth, height / sourceHeight);
  SVGtoPDF(doc, svg, 0, 0, { width: sourceWidth, height: sourceHeight, assumePt: true });
  doc.restore();
}
await pdf('logos/grove-color.pdf', 360, 360, (doc) => doc.image(source, 0, 0, { width: 360, height: 360 }));
await pdf('logos/grove-monochrome.pdf', 360, 360, (doc) => drawSvg(doc, monochrome, 360, 360));
await pdf('logos/grove-wordmark-light.pdf', 600, 210, (doc) => drawSvg(doc, wordmark(ink, white), 600, 210, 600, 210));
await pdf('menu-bar/GroveTemplate.pdf', 22, 22, (doc) => drawSvg(doc, monochrome, 22, 22));

await save('social/avatar-1024.png', await png(source, 1024));
await save('social/avatar-512.png', await png(source, 512));
const socialSvg = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="1200" height="630"><rect width="1200" height="630" fill="${ink}"/><image x="72" y="151" width="328" height="328" xlink:href="${uri(source)}"/><text x="454" y="270" fill="#ffffff" font-family="Arial,sans-serif" font-size="80" font-weight="bold">Grove</text><text x="457" y="340" fill="#e8ebed" font-family="Arial,sans-serif" font-size="30">Parallel dev environments</text><text x="457" y="385" fill="#3fdb84" font-family="Arial,sans-serif" font-size="30">from git worktrees.</text></svg>`;
await save('social/social-card-1200x630.png', await sharp(Buffer.from(socialSvg)).png().toBuffer());
await save('social/social-card-1200x630.jpg', await sharp(Buffer.from(socialSvg)).jpeg({ quality: 94 }).toBuffer());

const assets = [];
for (const relative of inventory.sort()) {
  const data = await readFile(join(brand, relative));
  let dimensions;
  if (/\.(png|jpg|webp|tiff)$/.test(relative)) {
    const meta = await sharp(data).metadata();
    dimensions = { width: meta.width, height: meta.height, alpha: meta.hasAlpha };
  }
  assets.push({ path: relative, bytes: data.length, sha256: createHash('sha256').update(data).digest('hex'), ...dimensions });
}
await writeFile(join(brand, 'manifest.json'), JSON.stringify({ name: 'Grove', selected: 'Original Grove C Git companion', approvedSource: 'sources/grove-c-approved.png', approvedSourceSha256: checksum, colorSvg: 'Raster-backed; approved colour image embedded losslessly. Monochrome SVGs contain vector paths.', generatedBy: 'pnpm --dir site brand:build', assets }, null, 2) + '\n');
const sizes = [16, 24, 32, 48, 64, 128];
const preview = `<!doctype html><html lang="en"><meta charset="utf-8"><title>Grove identity exports</title><style>*{box-sizing:border-box}body{margin:0;padding:40px;font:16px system-ui;background:#eef0f0;color:#15231c}h1{margin:0 0 8px;font-size:28px}p{margin:0 0 24px;color:#526056}section{display:flex;gap:24px;flex-wrap:wrap;margin-bottom:24px}.card{background:white;padding:24px;border-radius:16px;min-width:210px}.card img{display:block;width:170px;height:170px;object-fit:contain}.card span{display:block;margin-top:12px}.dark{background:#111b17;color:white}.sizes{display:flex;align-items:center;gap:22px;height:145px}.sizes figure{margin:0;text-align:center;font-size:12px}.sizes img{display:block;margin:auto auto 8px}.wordmark{width:380px!important;height:135px!important}.checker{background-color:#fff;background-image:linear-gradient(45deg,#e5e9e6 25%,transparent 25%),linear-gradient(-45deg,#e5e9e6 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#e5e9e6 75%),linear-gradient(-45deg,transparent 75%,#e5e9e6 75%);background-size:20px 20px;background-position:0 0,0 10px,10px -10px,-10px 0px}</style><h1>Grove · approved identity</h1><p>Original Grove C artwork. Production exports and small-size checks.</p><section><div class="card"><img src="../sources/grove-c-approved.png"><span>Approved original</span></div><div class="card"><img src="../app-icons/desktop/grove-512.png"><span>Desktop app icon</span></div><div class="card"><img src="../app-icons/png/grove-512.png"><span>Unmasked mobile / web</span></div><div class="card checker"><img src="../logos/grove-transparent.png"><span>Transparent companion</span></div></section><section><div class="card"><img src="../logos/grove-monochrome.svg"><span>Vector monochrome</span></div><div class="card dark"><img src="../logos/grove-white.svg"><span>Inverse monochrome</span></div><div class="card"><img class="wordmark" src="../logos/grove-wordmark-light.svg"><span>Wordmark</span></div></section><section><div class="card"><span>Browser icons · actual CSS pixels</span><div class="sizes">${sizes.map(n=>`<figure><img src="../app-icons/png/grove-${n}.png" style="width:${n}px;height:${n}px">${n}px</figure>`).join('')}</div></div><div class="card dark"><span>Menu-bar glyph · actual CSS pixels</span><div class="sizes">${[16,18,20,22,24,32].map(n=>`<figure><img src="../menu-bar/png/GroveWhite-${n}@2x.png" style="width:${n}px;height:${n}px">${n}pt</figure>`).join('')}</div></div></section></html>`;
await writeFile(join(brand, 'preview/index.html'), preview);
execFileSync('zip', ['-q', '-r', '-FS', 'Grove-Brand-Assets.zip', 'README.md', 'VERIFICATION.md', 'manifest.json', 'sources', 'logos', 'app-icons', 'web', 'menu-bar', 'social', 'preview'], { cwd: brand });
await install('public/brand/Grove-Brand-Assets.zip', await readFile(join(brand, 'Grove-Brand-Assets.zip')));
await writeFile(join(site, 'brand-assets.json'), JSON.stringify({
  schemaVersion: 1,
  approvedSourceSha256: checksum,
  files: siteAssets.sort((a, b) => a.path.localeCompare(b.path)),
}, null, 2) + '\n');
console.log(`Exported ${assets.length} assets from approved source ${checksum}.`);
