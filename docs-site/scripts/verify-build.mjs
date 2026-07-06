import { access, readFile } from 'node:fs/promises';

const root = new URL('../', import.meta.url);
const base = process.env.EXPECTED_BASE ?? '';
const out = new URL('out/', root);
const index = await readFile(new URL('index.html', out), 'utf8');

for (const route of ['docs/index.html', 'docs/handlers/index.html', 'docs/operations/reliability/index.html', 'api/search']) {
  await access(new URL(route, out));
}
if (!index.includes(`${base}/docs`)) throw new Error(`landing page does not link through base ${base || '/'}`);
if (base && index.includes('href="/docs')) throw new Error('root-relative docs link bypasses basePath');

console.log(`verified Fumadocs static export with base ${base || '/'}`);
