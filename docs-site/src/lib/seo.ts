import type { Metadata } from 'next';

export const site = {
  name: 'MailRelay',
  title: 'MailRelay — 可审计的邮件远程操作',
  description: '用认证邮件触发受限 Command，并持久化去重、审计、重试、dead letter 与回复投递。',
  url: 'https://liyown.github.io/MailRelay',
  github: 'https://github.com/liyown/MailRelay',
  locale: 'zh_CN',
};

export const keywords = [
  'MailRelay',
  '邮件自动化',
  '邮件远程操作',
  'Email automation',
  'IMAP automation',
  'SMTP reply',
  'Command protocol',
  '运维自动化',
  'workflow queue',
  'webhook',
  'MCP',
];

export function absoluteUrl(path = '/') {
  const clean = path.startsWith('/') ? path : `/${path}`;
  return `${site.url.replace(/\/+$/, '')}${clean}`;
}

export function pageMetadata(input: {
  title: string;
  description?: string;
  path?: string;
  type?: 'website' | 'article';
}): Metadata {
  const url = absoluteUrl(input.path ?? '/');
  const description = input.description || site.description;
  return {
    title: input.title,
    description,
    keywords,
    alternates: { canonical: url },
    openGraph: {
      title: input.title,
      description,
      url,
      siteName: site.name,
      locale: site.locale,
      type: input.type ?? 'website',
    },
    twitter: {
      card: 'summary',
      title: input.title,
      description,
    },
  };
}

export function jsonLd(data: Record<string, unknown>) {
  return JSON.stringify(data).replaceAll('<', '\\u003c');
}
