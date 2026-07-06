# Fumadocs Migration Design

## Goal

Replace the Astro/Starlight documentation implementation with the same core stack used by Nub: Next.js App Router, React, Fumadocs UI/MDX, Tailwind CSS, and Shiki code highlighting.

## Product design

- Preserve the current MailRelay landing-page message, Chinese documentation, warm paper palette, and GitHub Pages URL.
- Use Fumadocs native `DocsLayout`, `DocsPage`, mobile TOC, pagination, copy controls, and page tree instead of simulating them through Starlight overrides.
- Follow Nub's visual system: Encode Sans, compact 310px sidebar, 720px prose measure, sandstone borders, warm charcoal code panels, coral accent, and restrained controls.
- Keep MailRelay identity and content; do not copy Nub branding, promotional claims, or decorative star prompts.

## Architecture

- `docs-site/src/app` owns the Next.js App Router landing and documentation routes.
- `docs-site/content/docs` remains the single documentation source.
- `fumadocs-mdx` generates the typed content source; `fumadocs-core` builds the page tree.
- Search uses Fumadocs static Orama indexes so GitHub Pages needs no server.
- Next.js emits `docs-site/out` with `output: 'export'`, `trailingSlash: true`, and a GitHub Actions-only `/MailRelay` base path.

## Constraints

- Static GitHub Pages deployment must continue to work.
- All existing documentation routes must remain available.
- No server actions, middleware, image optimizer, or runtime API dependency.
- Mobile and desktop visual QA must pass before replacing the live site.

