import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Provider } from '@/components/provider';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL('https://liyown.github.io/MailRelay/'),
  title: { default: 'MailRelay — 可审计的邮件远程操作', template: '%s | MailRelay' },
  description: '用认证邮件触发受限 Command，并持久化去重、审计、重试与回复投递。',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body><Provider>{children}</Provider></body>
    </html>
  );
}
