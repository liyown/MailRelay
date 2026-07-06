# Fumadocs Migration Design QA

- Source visual truth: `https://nubjs.com/docs`, `/Users/liuyaowen/Documents/MailRelay/design-reference/nubjs-docs-desktop.png`, and the user-provided mobile screenshot `/tmp/codex-remote-attachments/019f385e-2407-7330-86ec-9a6245047902/E57E1C22-1B2D-4757-B63F-E5696B4FCEF7/2-照片-2.jpg`
- Implementation screenshots: `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-fumadocs-desktop.png`, `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-fumadocs-mobile.png`, and `/Users/liuyaowen/Documents/MailRelay/design-reference/mailrelay-fumadocs-landing.png`
- Full-view comparisons: `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-fumadocs-desktop.jpg` and `/Users/liuyaowen/Documents/MailRelay/design-reference/qa-fumadocs-mobile.jpg`
- Viewports: 1440 × 900 desktop; 390 × 844 mobile at 2× device scale
- States: documentation page, code block with copy control, mobile TOC, pagination, static search with Chinese query, and landing hero

## Findings

- No actionable P0/P1/P2 findings remain.
- Typography: Encode Sans is shared with Nub and now flows through Fumadocs rather than Starlight. Heading, prose, sidebar, and mono scales match Nub's compact hierarchy.
- Spacing and layout: native Fumadocs produces the same three-column documentation structure, 310px sidebar behavior, centered 720px prose measure, native mobile TOC, and compact pagination.
- Colors and tokens: MailRelay keeps Nub's warm paper, sandstone rules, coral accent, and charcoal Vesper code surface while retaining its own brand name and content.
- Image and icon quality: the documentation shell uses Fumadocs' production icons and controls; no source imagery is replaced with approximations. The black `N` visible in local screenshots is the Next.js development indicator and is absent from the static production export.
- Copy and content: all Chinese MailRelay documentation and routes are preserved. Nub product wording and decorative prompts were intentionally not copied.
- Interaction: copy control reports `Copied Text`; mobile and desktop widths have zero horizontal overflow; Chinese query `安全` returns indexed results through static Orama search.

## Focused comparison

Focused comparison is covered by the mobile composite: the TOC, copy button, code contrast, and pagination are readable at the same scale as the reported defects. Desktop comparison validates the shared Fumadocs frame and density.

## Patches made

- Replaced Astro/Starlight with Next.js App Router and Fumadocs UI/MDX.
- Added native Fumadocs DocsLayout, DocsPage, page tree, mobile TOC, copy control, and pagination.
- Added static Orama search with the Mandarin tokenizer.
- Added Next.js static export and GitHub Pages base-path handling.
- Rebuilt the landing route in React and preserved the existing MailRelay product narrative.

final result: passed
