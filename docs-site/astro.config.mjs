import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const repository = process.env.GITHUB_REPOSITORY?.split('/')[1];
const base = process.env.GITHUB_ACTIONS === 'true' && repository ? `/${repository}/` : '/';

export default defineConfig({
  site: process.env.SITE_URL ?? 'https://becomeopc.github.io',
  base,
  output: 'static',
  integrations: [
    starlight({
      title: 'mailrelay.',
      description: '把 Email 变成安全、可发现、可审计的命令协议。',
      customCss: ['./src/styles/docs.css'],
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/becomeopc/opc-mailrelay' }],
      sidebar: [
        { label: '开始', items: [
          { label: '介绍', slug: 'docs' },
          { label: '安装', slug: 'docs/getting-started/installation' },
          { label: '配置', slug: 'docs/getting-started/configuration' },
          { label: '第一个命令', slug: 'docs/getting-started/first-command' },
        ]},
        { label: '核心概念', items: [
          { label: '架构', slug: 'docs/concepts/architecture' },
          { label: 'Discovery', slug: 'docs/concepts/discovery' },
          { label: '安全模型', slug: 'docs/concepts/security' },
          { label: 'SQLite 持久化', slug: 'docs/concepts/storage' },
        ]},
        { label: 'Handlers', items: [
          { label: '概览', slug: 'docs/handlers' },
          { label: 'HTTP 与 Webhook', slug: 'docs/handlers/http-webhook' },
          { label: 'Workflow 与 Queue', slug: 'docs/handlers/workflow-queue' },
          { label: 'Plugin 与 Shell', slug: 'docs/handlers/plugin-shell' },
          { label: 'Agent 与 MCP', slug: 'docs/handlers/agent-mcp' },
        ]},
        { label: '运维', items: [
          { label: 'CLI', slug: 'docs/operations/cli' },
          { label: '可靠性与恢复', slug: 'docs/operations/reliability' },
          { label: 'GitHub Pages', slug: 'docs/operations/github-pages' },
        ]},
      ],
    }),
  ],
});
