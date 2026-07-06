import { access, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const root = new URL('..', import.meta.url).pathname;
const base = process.env.EXPECTED_BASE ?? '/';
const index = await readFile(join(root, 'dist/index.html'), 'utf8');

for (const route of ['docs/index.html', 'docs/handlers/index.html', 'docs/operations/reliability/index.html']) {
  await access(join(root, 'dist', route));
}

if (!index.includes(`href="${base}docs/"`)) {
  throw new Error(`landing page does not link to ${base}docs/`);
}

if (index.includes('href="/docs/"') && base !== '/') {
  throw new Error('root-relative docs link bypasses the GitHub Pages base path');
}

console.log(`verified static build with base ${base}`);
