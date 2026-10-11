import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { dirname, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

// Deployment guard: only Node built-ins and files inside site/. It deliberately
// does not require the repository's native export tools or an iCloud bridge.
const site = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const required = [
  'src/app/favicon.ico', 'src/app/icon.svg', 'src/app/apple-icon.png',
  'public/brand/grove-icon.png', 'public/brand/Grove-Brand-Assets.zip',
  'public/brand/safari-pinned-tab.svg',
  'public/brand/icon-192.png', 'public/brand/icon-512.png',
  'public/brand/maskable-192.png', 'public/brand/maskable-512.png',
];

try {
  const manifest = JSON.parse(await readFile(resolve(site, 'brand-assets.json'), 'utf8'));
  if (manifest.schemaVersion !== 1 || !Array.isArray(manifest.files)) {
    throw new Error('Invalid brand-assets.json');
  }
  const paths = new Set(manifest.files.map(file => file.path));
  for (const path of required) {
    if (!paths.has(path)) throw new Error(`Required branding file is not registered: ${path}`);
  }
  for (const file of manifest.files) {
    const path = resolve(site, file.path);
    if (!path.startsWith(site + sep)) throw new Error(`Branding file is outside site/: ${file.path}`);
    const bytes = await readFile(path);
    if (bytes.length !== file.bytes || createHash('sha256').update(bytes).digest('hex') !== file.sha256) {
      throw new Error(`Branding file differs from the exported source: ${file.path}`);
    }
  }
  console.log(`Verified ${manifest.files.length} site branding files, including the downloadable pack.`);
} catch (error) {
  console.error(`Grove branding check failed: ${error.message}`);
  console.error('Restore the exported files, or run pnpm --dir site brand:build from the full Grove checkout on macOS.');
  process.exitCode = 1;
}
