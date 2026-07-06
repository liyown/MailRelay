import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { rehypeCodeDefaultOptions } from 'fumadocs-core/mdx-plugins';

export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    postprocess: { includeProcessedMarkdown: true },
  },
});

export default defineConfig({
  mdxOptions: {
    rehypeCodeOptions: {
      themes: { light: 'vesper', dark: 'vesper' },
      transformers: rehypeCodeDefaultOptions.transformers,
    },
  },
});
