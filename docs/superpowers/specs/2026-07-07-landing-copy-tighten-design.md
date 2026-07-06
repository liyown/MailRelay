# Landing Copy Tightening Design

## Objective

Make the existing long-form landing page more professional, concise, and visually calm without removing its core product narrative.

## Scope

- Keep the current section order, responsive layout, and component structure.
- Keep golden scenarios, email advantages, Discovery, protocol, comparison, security, handler maturity, and quick start.
- Reduce visible prose by roughly 30–40%.
- Preserve concrete product and security facts; remove repeated promises and explanatory padding.

## Editorial Rules

- One claim per heading and one supporting sentence per section.
- Scenario descriptions use direct operational language and remain under 55 Chinese characters where practical.
- Prefer product nouns and verbs over conversational framing.
- Avoid repeated claims such as “无需 App”, “不重新发明”, and “不是一句标语”.
- Use English only for established product concepts, protocol names, maturity labels, and short editorial labels.
- Do not introduce claims that cannot be supported by the current implementation.

## Visual Rules

- Preserve the warm editorial visual system and existing dark proof surfaces.
- Remove copy-driven vertical bulk; do not compensate with new cards or decoration.
- Keep the page readable at 390 px without document-level horizontal overflow.

## Acceptance Criteria

- All existing content verification phrases remain present.
- The landing page has the same ten major sections and four golden scenarios.
- `pnpm check`, static export validation for `/MailRelay`, and Go tests pass.
- Desktop and mobile screenshots show no clipped copy, broken cards, or horizontal page overflow.
