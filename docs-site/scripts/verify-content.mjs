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
const seo = await readFile(new URL('src/lib/seo.ts', root), 'utf8');
const publicCopy = `${landing}\n${metadata}\n${seo}`;
const handlerOverview = await readFile(new URL('content/docs/handlers/index.mdx', root), 'utf8');
const workflowQueue = await readFile(new URL('content/docs/handlers/workflow-queue.mdx', root), 'utf8');
const reliability = await readFile(new URL('content/docs/operations/reliability.mdx', root), 'utf8');
for (const phrase of [
  '让能发邮件的客户端调用受控 API',
  'MailRelay — 用邮件触发受控 API 和命令',
  'ChatGPT 手机端调用外部 API',
  '收邮件、验身份、跑命令、写记录、回信',
  '把 ChatGPT 手机端、快捷指令、NAS 脚本或任意邮件客户端接到你配置好的 HTTP',
  'mailrelay run',
  'Use cases',
  '从手机、ChatGPT 或脚本发一封邮件',
  'Email as input',
  'Command catalog',
  'Where it fits',
  'Runtime checks',
  'Try it',
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
  '稳定性不是插件',
  'Golden scenarios',
  'First stable command',
  '四个适合邮件触发的运维场景',
]) {
  if (publicCopy.includes(phrase)) throw new Error(`landing page contains filler copy: ${phrase}`);
}

for (const phrase of [
  '| Workflow | Stable |',
  '| Queue | Stable |',
]) {
  if (!handlerOverview.includes(phrase)) throw new Error(`handler overview is missing: ${phrase}`);
}

for (const phrase of [
  '间接循环',
  '最大深度',
  '第一步失败后停止',
  '不能声明敏感参数',
  'unknown_command',
  'invalid_parameters',
]) {
  if (!workflowQueue.includes(phrase)) throw new Error(`workflow/queue guide is missing: ${phrase}`);
}

for (const phrase of [
  'mailrelay replay queue 42',
  '依赖故障',
  '终止错误',
]) {
  if (!reliability.includes(phrase)) throw new Error(`reliability guide is missing: ${phrase}`);
}

console.log(`verified ${required.length} Fumadocs content artifacts`);
