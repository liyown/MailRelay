import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Provider } from '@/components/provider';
import { absoluteUrl, keywords, site } from '@/lib/seo';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL(`${site.url}/`),
  applicationName: site.name,
  title: { default: site.title, template: '%s | MailRelay' },
  description: site.description,
  keywords,
  authors: [{ name: 'becomeopc' }],
  creator: 'becomeopc',
  publisher: 'becomeopc',
  category: 'developer tools',
  alternates: { canonical: absoluteUrl('/') },
  openGraph: {
    type: 'website',
    locale: site.locale,
    url: absoluteUrl('/'),
    siteName: site.name,
    title: site.title,
    description: site.description,
  },
  twitter: {
    card: 'summary',
    title: site.title,
    description: site.description,
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body><Provider>{children}</Provider></body>
    </html>
  );
}
