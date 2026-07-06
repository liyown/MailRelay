# Landing Copy Tightening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce landing-page prose by 30–40% while preserving all ten sections, four scenarios, and verifiable product claims.

**Architecture:** Keep the current React and CSS structure. Tighten only content arrays and visible copy, then enforce required concepts and removed filler phrases in the existing static content verifier.

**Tech Stack:** Next.js 16, React 19, TypeScript, static verification scripts, Go test suite.

## Global Constraints

- Keep the current section order and responsive component structure.
- Preserve ten major sections and four golden scenarios.
- Do not add unsupported product claims or visual decoration.
- Preserve a 390 px layout without document-level horizontal overflow.

---

### Task 1: Add editorial regression checks

**Files:**
- Modify: `docs-site/scripts/verify-content.mjs`

**Interfaces:**
- Consumes: rendered landing source at `docs-site/src/app/page.tsx`
- Produces: build-time rejection of known filler phrases and missing core concepts

- [ ] Add forbidden phrase checks for repeated conversational copy.
- [ ] Run `pnpm check` and confirm it fails on the current landing copy.
- [ ] Commit the failing editorial test with the implementation in Task 2.

### Task 2: Tighten landing-page copy

**Files:**
- Modify: `docs-site/src/app/page.tsx`

**Interfaces:**
- Consumes: existing arrays and presentational components
- Produces: the same page structure with shorter professional copy

- [ ] Rewrite hero, scenarios, section introductions, proof text, comparison text, trust claims, handler descriptions, and CTA.
- [ ] Remove duplicate promises and conversational qualifiers.
- [ ] Run `pnpm check` and confirm editorial checks pass.
- [ ] Run the static export with `GITHUB_ACTIONS=true GITHUB_REPOSITORY=liyown/MailRelay pnpm build`.
- [ ] Run `EXPECTED_BASE=/MailRelay pnpm check:build`.

### Task 3: Visual and repository verification

**Files:**
- Modify: `design-qa.md`

**Interfaces:**
- Consumes: local production-equivalent page
- Produces: desktop/mobile QA evidence and final approval record

- [ ] Verify desktop hierarchy and 390 px document width in Chrome.
- [ ] Confirm ten sections and four scenarios remain.
- [ ] Update `design-qa.md` with the editorial QA result.
- [ ] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and `git diff --check`.
- [ ] Commit, merge to `main`, push, and verify GitHub Pages returns the tightened copy.
