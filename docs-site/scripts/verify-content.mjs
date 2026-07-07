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
const metadata = await readFile(new URL('src/app/layout.tsx', root), 'utf8');
const publicCopy = `${landing}\n${metadata}`;
for (const phrase of [
  '用邮件执行可审计的远程操作',
  'MailRelay — 可审计的邮件远程操作',
  '用认证邮件触发受限 Command',
  '稳定性不是插件，是运行时边界',
  '去重、审计、重试、dead letter 与回复投递都由 SQLite 持久化',
  'mailrelay run',
  'Golden scenarios',
  '四个适合邮件触发的运维场景',
  'Why email works',
  'Command catalog',
  'MailRelay vs. the usual remote controls',
  'Runtime safety',
  'First stable command',
]) {
  if (!publicCopy.includes(phrase)) throw new Error(`landing page is missing: ${phrase}`);
}

for (const phrase of [
  'Email is the command line',
  'The universal remote for everything you run',
  'The big promise',
  'everything you run',
  'Built to be safe by default',
  '不是高频交互界面',
  '不重新发明控制通道',
  '不是一句标语',
  '不要先造平台',
]) {
  if (publicCopy.includes(phrase)) throw new Error(`landing page contains filler copy: ${phrase}`);
}

console.log(`verified ${required.length} Fumadocs content artifacts`);
