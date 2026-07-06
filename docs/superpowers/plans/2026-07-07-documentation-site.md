# MailRelay Documentation Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and deploy a polished static MailRelay documentation site.

**Architecture:** Astro Starlight generates a multi-page static site from Markdown. Custom theme CSS combines Nub's warm editorial palette with Fumadocs' documentation layout, while GitHub Actions publishes the build to GitHub Pages.

**Tech Stack:** Astro, Starlight, Markdown, CSS, pnpm, GitHub Actions.

---

### Task 1: Site scaffold and theme

- [ ] Create `docs-site` package, Astro config, Starlight navigation, and custom CSS.
- [ ] Build once to verify the static toolchain.

### Task 2: Documentation content

- [ ] Write introduction, installation, configuration, first-command, architecture, Discovery, security, storage, handler, CLI, reliability, and deployment pages.
- [ ] Cross-check commands and configuration fields against current source.

### Task 3: GitHub Pages

- [ ] Add Pages deployment workflow and repository-base handling.
- [ ] Validate the build with a non-root base path.

### Task 4: Visual QA

- [ ] Run the site locally and capture desktop/mobile screenshots.
- [ ] Compare source and implementation evidence, fix P0-P2 differences, and save `design-qa.md` with `final result: passed`.

### Task 5: Verification

- [ ] Run the docs build, internal link checks, existing Go tests, and git diff checks.
