import { access, readFile } from 'node:fs/promises';

const root = new URL('../', import.meta.url);
const required = [
  'content/docs/index.mdx',
  'content/docs/getting-started/installation.mdx',
  'content/docs/getting-started/configuration.mdx',
  'content/docs/concepts/discovery.mdx',
  'content/docs/concepts/security.mdx',
  'content/docs/handlers/index.mdx',
  'content/docs/operations/cli.mdx',
  'content/docs/operations/reliability.mdx',
  'content/docs/operations/github-pages.mdx',
];
for (const file of required) await access(new URL(file, root));

const landing = await readFile(new URL('src/app/page.tsx', root), 'utf8');
for (const phrase of [
  'Email is the command line',
  'Built to be safe by default',
  'mailrelay run',
  'Golden scenarios',
  'The universal remote for everything you run',
  'Why email wins',
  'Discovery is the interface',
  'MailRelay vs. the usual remote controls',
  'Trust is a runtime feature',
  'Start with one command',
]) {
  if (!landing.includes(phrase)) throw new Error(`landing page is missing: ${phrase}`);
}

console.log(`verified ${required.length} Fumadocs content artifacts`);
