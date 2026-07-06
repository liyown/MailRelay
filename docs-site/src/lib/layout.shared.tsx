import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: <span className="mailrelay-wordmark">mailrelay.</span>,
      transparentMode: 'none',
    },
    githubUrl: 'https://github.com/liyown/MailRelay',
    links: [
      { text: '文档', url: '/docs', active: 'nested-url' },
      { text: 'GitHub', url: 'https://github.com/liyown/MailRelay', external: true },
    ],
  };
}
