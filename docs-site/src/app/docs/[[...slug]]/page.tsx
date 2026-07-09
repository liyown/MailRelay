import { notFound } from 'next/navigation';
import { DocsBody, DocsDescription, DocsPage, DocsTitle } from 'fumadocs-ui/page';
import { source } from '@/lib/source';
import { absoluteUrl, jsonLd, pageMetadata, site } from '@/lib/seo';
import { getMDXComponents } from '../../../../mdx-components';

export default async function Page({ params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug } = await params;
  const page = source.getPage(slug);
  if (!page) notFound();
  const MDX = page.data.body;
  const structuredData = {
    '@context': 'https://schema.org',
    '@type': 'TechArticle',
    headline: page.data.title,
    description: page.data.description,
    url: absoluteUrl(page.url),
    inLanguage: 'zh-CN',
    isPartOf: {
      '@type': 'WebSite',
      name: site.name,
      url: absoluteUrl('/'),
    },
  };
  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd(structuredData) }} />
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription>{page.data.description}</DocsDescription>
      <DocsBody><MDX components={getMDXComponents()} /></DocsBody>
    </DocsPage>
  );
}

export function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata({ params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug } = await params;
  const page = source.getPage(slug);
  if (!page) notFound();
  return pageMetadata({
    title: page.data.title,
    description: page.data.description,
    path: page.url,
    type: 'article',
  });
}
