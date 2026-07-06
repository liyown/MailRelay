# Mobile Documentation Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refresh the mobile TOC, code copy control, code readability, and pagination while preserving Starlight behavior.

**Architecture:** Add scoped CSS overrides in the existing Starlight custom stylesheet. Extend the content verifier with a style-contract check, then validate the generated static site and compare a 390px render against the supplied screenshots.

**Tech Stack:** Astro 7, Starlight 0.41, Expressive Code, CSS, Node.js verifier.

## Global Constraints

- No new runtime JavaScript or dependencies.
- Preserve all native controls and navigation semantics.
- Keep the Nub-inspired warm palette.

---

### Task 1: Style contract

**Files:**
- Modify: `docs-site/scripts/verify-content.mjs`
- Test: `pnpm check`

- [ ] Add assertions for the mobile TOC, copy button, readable code palette, and flat pagination selectors.
- [ ] Run `pnpm check` and confirm it fails because the selectors are absent.

### Task 2: Responsive visual implementation

**Files:**
- Modify: `docs-site/src/styles/docs.css`

- [ ] Add the minimal scoped overrides for the four required surfaces.
- [ ] Run `pnpm check` and confirm the style contract passes.
- [ ] Run `pnpm build` and `EXPECTED_BASE=/ pnpm check:build`.

### Task 3: Visual QA and release

**Files:**
- Modify: `design-qa.md`

- [ ] Capture the architecture page at 390px with the TOC closed and open.
- [ ] Compare source and implementation evidence, fix all P0-P2 findings, and record `final result: passed`.
- [ ] Commit, push `main`, wait for GitHub Pages, and verify the deployed URL.

