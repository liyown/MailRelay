# Landing Product Story Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the MailRelay homepage into a long-form, credible product narrative with golden use cases, category claims, proof, comparison, Discovery, and conversion sections.

**Architecture:** Keep the existing Next.js route and Nub-inspired CSS system. Extract reusable product data into typed arrays, render each narrative section from that data, and extend the static verifier with required product-story contracts.

**Tech Stack:** Next.js 16, React 19, TypeScript, Tailwind/Fumadocs global CSS.

## Global Constraints

- Do not invent benchmarks, customer claims, testimonials, or security guarantees.
- Every product claim must map to an implemented MailRelay capability.
- Preserve existing landing links, documentation routes, and GitHub Pages static export.
- Mobile width must not overflow at 390px.

---

### Task 1: Product-story contract

**Files:**
- Modify: `docs-site/scripts/verify-content.mjs`

- [ ] Assert the landing source includes golden scenarios, the category claim, Discovery demonstration, alternatives comparison, trust proof, and final CTA.
- [ ] Run `pnpm check` and confirm it fails on the current short landing page.

### Task 2: Long-form landing content

**Files:**
- Modify: `docs-site/src/app/page.tsx`
- Modify: `docs-site/src/app/global.css`

- [ ] Add the golden scenario sequence and email mockups.
- [ ] Add the category manifesto, why-email proof, Discovery demonstration, comparison table, trust proof, Handler horizon, and five-minute close.
- [ ] Add responsive editorial layouts, mobile stacking, code panels, and section navigation.
- [ ] Run `pnpm check`, static export, and route verification.

### Task 3: Visual QA and deployment

**Files:**
- Modify: `design-qa.md`

- [ ] Capture full desktop and 390px mobile pages.
- [ ] Compare the implementation with the existing Nub landing reference and fix all P0-P2 issues.
- [ ] Verify links and zero horizontal overflow.
- [ ] Commit, merge, push, wait for Pages deployment, and verify the live homepage.

