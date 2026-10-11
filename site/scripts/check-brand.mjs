import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import sharp from 'sharp';

const site = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const brand = resolve(site, '../brand');
const manifest = JSON.parse(await readFile(join(brand, 'manifest.json'), 'utf8'));
const deployed = JSON.parse(await readFile(join(site, 'brand-assets.json'), 'utf8'));
assert.equal(deployed.approvedSourceSha256, manifest.approvedSourceSha256, 'Website branding belongs to a different source export');
const sha = data => createHash('sha256').update(data).digest('hex');
assert.equal(sha(await readFile(join(brand, manifest.approvedSource))), manifest.approvedSourceSha256);
for (const asset of manifest.assets) {
  const data = await readFile(join(brand, asset.path));
  assert.equal(sha(data), asset.sha256, `${asset.path}: stale hash`);
  if (asset.width) {
    const meta = await sharp(data).metadata();
    assert.equal(meta.width, asset.width, asset.path);
    assert.equal(meta.height, asset.height, asset.path);
  }
}
for (const platform of ['macos', 'ios']) {
  const dir = join(brand, `app-icons/${platform}/Assets.xcassets/AppIcon.appiconset`);
  const catalog = JSON.parse(await readFile(join(dir, 'Contents.json'), 'utf8'));
  for (const entry of catalog.images) {
    const meta = await sharp(join(dir, entry.filename)).metadata();
    assert.equal(meta.width, parseFloat(entry.size) * parseFloat(entry.scale), entry.filename);
    assert.equal(meta.height, meta.width, entry.filename);
    if (platform === 'ios') assert.equal(meta.hasAlpha, false, `${entry.filename}: iOS must be opaque`);
  }
}
for (const [file, expected] of [['web/favicon.ico', [16, 32, 48, 64]], ['app-icons/windows/Grove.ico', [16, 24, 32, 48, 64, 128, 256]]]) {
  const data = await readFile(join(brand, file));
  assert.equal(data.readUInt16LE(0), 0);
  assert.equal(data.readUInt16LE(2), 1);
  const count = data.readUInt16LE(4);
  assert.equal(count, expected.length, file);
  for (let n = 0; n < count; n++) {
    const i = 6 + n * 16;
    assert.equal(data[i] || 256, expected[n], file);
    const bytes = data.readUInt32LE(i + 8), offset = data.readUInt32LE(i + 12);
    assert.ok(bytes > 0 && offset + bytes <= data.length, `${file}: invalid image directory`);
  }
}
const icns = await readFile(join(brand, 'app-icons/macos/Grove.icns'));
assert.equal(icns.toString('ascii', 0, 4), 'icns');
assert.equal(icns.readUInt32BE(4), icns.length);
let offset = 8, chunks = 0;
while (offset < icns.length) {
  const size = icns.readUInt32BE(offset + 4);
  assert.ok(size >= 8 && offset + size <= icns.length);
  offset += size; chunks++;
}
assert.equal(offset, icns.length);
assert.ok(chunks >= 8);
for (const file of ['logos/grove-monochrome.svg', 'logos/grove-white.svg', 'menu-bar/GroveTemplate.svg']) {
  const svg = await readFile(join(brand, file), 'utf8');
  assert.ok(svg.includes('<path'), file);
  assert.ok(!svg.includes('<image'), `${file}: expected true vector`);
}
for (const file of ['app-icons/desktop/grove-1024.png', 'menu-bar/png/GroveTemplate-22@2x.png']) {
  const { data, info } = await sharp(join(brand, file)).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  assert.equal(data[3], 0, `${file}: transparent corner`);
  assert.equal(data[(info.width - 1) * 4 + 3], 0, `${file}: transparent corner`);
}
const template = await sharp(join(brand, 'menu-bar/png/GroveTemplate-22@2x.png')).raw().toBuffer({ resolveWithObject: true });
for (let i = 0; i < template.data.length; i += template.info.channels) {
  if (template.data[i + 3] > 0) assert.ok(template.data[i] === 0 && template.data[i + 1] === 0 && template.data[i + 2] === 0, 'Template must contain black ink only');
}
for (const [src, dest] of [['web/favicon.ico', 'src/app/favicon.ico'], ['web/apple-touch-icon.png', 'src/app/apple-icon.png']]) {
  assert.equal(sha(await readFile(join(brand, src))), sha(await readFile(join(site, dest))));
}
assert.ok((await readdir(join(brand, 'app-icons/android/res'))).includes('mipmap-anydpi-v26'));
console.log(`Verified ${manifest.assets.length} exported files; ICO/ICNS structures; Apple catalogs; vector templates; alpha; website copies.`);
