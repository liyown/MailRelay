import { access, readFile } from 'node:fs/promises';

const root = new URL('../', import.meta.url);
const pkg = JSON.parse(await readFile(new URL('package.json', root), 'utf8'));
const deps = { ...pkg.dependencies, ...pkg.devDependencies };

for (const name of ['next', 'react', 'fumadocs-core', 'fumadocs-ui', 'fumadocs-mdx', 'tailwindcss']) {
  if (!deps[name]) throw new Error(`missing Fumadocs stack dependency: ${name}`);
}
if (deps.astro || deps['@astrojs/starlight']) throw new Error('Astro/Starlight dependency remains');

const required = [
  'next.config.mjs',
  'source.config.ts',
  'src/app/layout.tsx',
  'src/app/page.tsx',
  'src/app/docs/layout.tsx',
  'src/app/docs/[[...slug]]/page.tsx',
  'src/lib/source.ts',
  'src/components/search.tsx',
  'mdx-components.tsx',
];
for (const file of required) await access(new URL(file, root));

const config = await readFile(new URL('next.config.mjs', root), 'utf8');
for (const token of ["output: 'export'", 'trailingSlash: true', "basePath"]) {
  if (!config.includes(token)) throw new Error(`next config missing: ${token}`);
}

const search = await readFile(new URL('src/components/search.tsx', root), 'utf8');
if (!search.includes('NEXT_PUBLIC_BASE_PATH')) {
  throw new Error('static search does not honor the GitHub Pages base path');
}

console.log('verified Next.js + Fumadocs static architecture');
