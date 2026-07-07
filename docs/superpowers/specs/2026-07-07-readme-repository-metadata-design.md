# README and Repository Metadata Design

## Objective

Present MailRelay as a professional open-source product within the first screen of its GitHub repository, while keeping detailed operational material in the documentation site.

## README Structure

1. Product name, one-line positioning, and links to Documentation and GitHub Pages.
2. Compact architecture flow and three core value propositions.
3. Five-minute quick start.
4. One configuration-driven command example.
5. Discovery behavior for `help` and `help deploy`.
6. Handler support matrix.
7. Security and reliability summary.
8. CLI summary, development commands, and documentation links.

## Editorial Rules

- English is the primary README language.
- Prefer short sections, tables, and examples over long prose.
- Do not duplicate the full handler reference or 72-hour acceptance procedure.
- Link detailed concepts, operations, and handler documentation to the deployed site.
- State only behavior implemented in the current repository.

## Repository Metadata

- Description: `Turn authenticated email into safe, auditable automation commands.`
- Homepage: `https://liyown.github.io/MailRelay/`
- Repository URL remains `https://github.com/liyown/MailRelay`.
- Local `origin` remains `https://github.com/liyown/MailRelay.git`.

## Acceptance Criteria

- README renders with a clear product introduction before setup details.
- Installation, configuration, Discovery, handlers, security, CLI, and development remain discoverable.
- Every documentation link resolves under `/MailRelay/docs`.
- Go tests pass.
- GitHub reports the exact description, homepage, and repository URL above.
