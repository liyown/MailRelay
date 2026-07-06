# Design QA

## Evidence

- Source visual truth:
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/nubjs-landing-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/nubjs-landing-mobile.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/fumadocs-ui-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/fumadocs-ui-mobile.png`
- Rendered implementation:
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-landing-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-landing-mobile.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-docs-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-docs-mobile.png`
- Full-view comparison evidence:
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-landing-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-landing-mobile.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-docs-desktop.png`
  - `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-docs-mobile.png`
- Viewports: desktop 2560×1177/1233; mobile 390×844.
- State: light theme, landing route `/`, introduction route `/docs/`.

## Findings

- No actionable P0, P1, or P2 findings remain.
- Typography: local Encode Sans Variable matches Nub's measured family and editorial weight closely. The MailRelay headline is intentionally shorter and heavier enough to remain legible with mixed Chinese/English copy.
- Spacing and layout rhythm: desktop hero keeps Nub's offset text/terminal composition, generous top whitespace, 58px sticky navigation, and warm bordered install panel. Mobile collapses to one column without horizontal page overflow.
- Colors and tokens: warm paper, deep brown-black, sandstone borders, coral accent, and black code surfaces match the captured reference palette. Documentation retains the same tokens inside Starlight's three-column structure.
- Image quality and assets: neither implementation screen requires illustration or product imagery. No source logo or proprietary asset was copied; the site uses a text wordmark and local open-source font package.
- Copy and content: all visible copy is MailRelay-specific and reflects current CLI, Handler maturity, SQLite Outbox, security, and recovery behavior.
- Documentation layout: desktop left navigation, focused article, right table of contents, and mobile single-column collapse match Fumadocs' captured structure. Code blocks are intentionally dark to maintain continuity with the Nub landing page.

## Patches Made During QA

- Added safe word-break opportunities to the long `go install` command.
- Reduced install command type size and allowed wrapping instead of clipping.
- Added trailing-slash GitHub Pages base handling.
- Replaced a root-relative documentation link with a route-relative link.
- Added static-build verification for normal and `/opc-mailrelay/` base paths.

## Focused Region Comparison

The hero was compared separately because typography, terminal sizing, install panel alignment, and whitespace determine most of the Nub-style fidelity. The final implementation keeps the same two-column silhouette on desktop and the same headline → description → install → terminal order on mobile.

## Follow-up Polish

- P3: Add a small MailRelay-specific illustration only if a future brand system provides an approved asset. The current text-and-terminal composition is intentionally asset-free.

final result: passed
