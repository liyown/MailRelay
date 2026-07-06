import { access, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const root = new URL('..', import.meta.url).pathname;
const required = [
  'src/pages/index.astro',
  'src/content/docs/docs/index.mdx',
  'src/content/docs/docs/getting-started/installation.mdx',
  'src/content/docs/docs/getting-started/configuration.mdx',
  'src/content/docs/docs/concepts/discovery.mdx',
  'src/content/docs/docs/concepts/security.mdx',
  'src/content/docs/docs/handlers/index.mdx',
  'src/content/docs/docs/operations/cli.mdx',
  'src/content/docs/docs/operations/reliability.mdx',
  'src/content/docs/docs/operations/github-pages.mdx',
  '../.github/workflows/docs-pages.yml',
];

for (const file of required) await access(join(root, file));

const landing = await readFile(join(root, 'src/pages/index.astro'), 'utf8');
for (const phrase of ['Email is the command line', 'Built to be safe by default', 'mailrelay run']) {
  if (!landing.includes(phrase)) throw new Error(`landing page is missing: ${phrase}`);
}

console.log(`verified ${required.length} documentation artifacts`);
