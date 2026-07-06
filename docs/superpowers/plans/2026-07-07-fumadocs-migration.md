# Fumadocs Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the MailRelay website with Nub's Next.js and Fumadocs stack while preserving content and GitHub Pages deployment.

**Architecture:** Replace Astro entrypoints with Next.js App Router routes and Fumadocs layouts. Generate a static export under the repository base path and use static Orama search.

**Tech Stack:** Next.js 16, React 19, Fumadocs 16, Fumadocs MDX 15, Tailwind CSS 4, Shiki, Orama.

## Global Constraints

- Preserve all existing documentation routes and content.
- Output a fully static `out` directory for GitHub Pages.
- Use `/MailRelay` only in GitHub Actions builds and `/` locally.
- Do not depend on runtime APIs or middleware.

---

### Task 1: Migration contract

**Files:**
- Create: `docs-site/scripts/verify-fumadocs.mjs`

- [ ] Assert the package uses Next.js, Fumadocs, Tailwind and contains no Astro dependency.
- [ ] Assert Next config enables static export and the required App Router/Fumadocs files exist.
- [ ] Run the verifier and confirm it fails against the existing Starlight project.

### Task 2: Next.js and Fumadocs application

**Files:**
- Replace: `docs-site/package.json`, `docs-site/astro.config.mjs`, `docs-site/src/pages/index.astro`
- Create: `docs-site/next.config.mjs`, `docs-site/source.config.ts`, `docs-site/src/app/**`, `docs-site/src/lib/**`, `docs-site/src/components/**`, `docs-site/mdx-components.tsx`
- Move: `docs-site/src/content/docs/docs/**` to `docs-site/content/docs/**`

- [ ] Add the static Next.js/Fumadocs shell and content loader.
- [ ] Add Fumadocs static search and Chinese tokenizer.
- [ ] Rebuild the landing page as a responsive React route.
- [ ] Implement Nub-inspired tokens, documentation density, and warm Vesper code treatment.
- [ ] Install dependencies and run the migration verifier.

### Task 3: Deployment and visual QA

**Files:**
- Modify: `.github/workflows/docs-pages.yml`
- Modify: `design-qa.md`

- [ ] Build locally and with the `/MailRelay` GitHub Pages base path.
- [ ] Verify every expected route exists in `out`.
- [ ] Capture desktop and 390px mobile screenshots, including mobile TOC and code controls.
- [ ] Compare against Nub and the user screenshots; fix all P0-P2 findings.
- [ ] Commit, merge, push, and verify the deployed Pages URL.

