# Changelog

All notable changes use semantic versioning and are documented here.

## [Unreleased]

## [0.1.0] - 2026-07-10

### Added

- Authenticated email-to-command routing with typed parameters, generated help, deduplication, and safe email replies.
- Durable SQLite reply outbox with independent SMTP retry.
- Dead-letter visibility and explicit queue/reply replay commands.
- Handler maturity labels in generated Discovery.
- Runtime health fields and a 72-hour soak command.
- Local TLS SMTP, scripted IMAP, hot-reload, and SSRF regression coverage.
- Version metadata, CI, cross-platform release archives, and checksums.
- Embedded operations console with command editing, execution history, runtime events, queue recovery, API Action templates, request previews, and read-only mail simulation.

### Security

- Doctor warnings for Beta and Experimental handlers.
- Doctor validation for declared executables and outbound DNS policy.
- HTTP-family handlers restrict destinations through an explicit host allowlist and redact sensitive request data from operational snapshots.
