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
- Repository URL: `https://github.com/liyown/MailRelay`.
- Local `origin` remains `https://github.com/liyown/MailRelay.git`.

## Go Module Path

- The Go module path is `github.com/liyown/MailRelay`, matching the repository URL.
- `go install github.com/liyown/MailRelay/cmd/mailrelay@latest` is the canonical install command.
- The previous `github.com/becomeopc/opc-mailrelay` module path is no longer published; v0.1 has no public downstream consumers.

## Documentation Landing Targets

The README links to the deployed site, not the repository path. All links resolve under `https://liyown.github.io/MailRelay/docs/`.

| Topic | Docs page |
| --- | --- |
| Five-minute quick start | `/docs/getting-started/installation` |
| First command walkthrough | `/docs/getting-started/first-command` |
| Configuration reference | `/docs/getting-started/configuration` |
| Architecture | `/docs/concepts/architecture` |
| Discovery | `/docs/concepts/discovery` |
| Security model | `/docs/concepts/security` |
| Storage and audit | `/docs/concepts/storage` |
| Handler reference | `/docs/handlers` |
| CLI reference | `/docs/operations/cli` |
| Reliability and 72-hour acceptance | `/docs/operations/reliability` |
| GitHub Pages deployment | `/docs/operations/github-pages` |

The 72-hour acceptance procedure now lives in `docs-site/content/docs/operations/reliability.mdx` so the README can stay a one-screen product card.

## Acceptance Criteria

- README renders with a clear product introduction before setup details.
- Installation, configuration, Discovery, handlers, security, CLI, and development remain discoverable.
- Every documentation link resolves under `/MailRelay/docs`.
- `go test ./...` and `go vet ./...` pass.
- GitHub reports the exact description, homepage, and repository URL above.
