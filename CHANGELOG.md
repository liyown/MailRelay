# Changelog

All notable changes use semantic versioning and are documented here.

## [Unreleased]

### Added

- Durable SQLite reply outbox with independent SMTP retry.
- Dead-letter visibility and explicit queue/reply replay commands.
- Handler maturity labels in generated Discovery.
- Runtime health fields and a 72-hour soak command.
- Local TLS SMTP, scripted IMAP, hot-reload, and SSRF regression coverage.
- Version metadata, CI, cross-platform release archives, and checksums.

### Security

- Doctor warnings for Beta and Experimental handlers.
- Doctor validation for declared executables and outbound DNS policy.
