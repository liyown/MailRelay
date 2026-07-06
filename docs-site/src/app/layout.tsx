import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Provider } from '@/components/provider';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL('https://liyown.github.io/MailRelay/'),
  title: { default: 'MailRelay — Email is the command line', template: '%s | MailRelay' },
  description: '把 Email 变成安全、可发现、可审计的命令协议。',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body><Provider>{children}</Provider></body>
    </html>
  );
}
