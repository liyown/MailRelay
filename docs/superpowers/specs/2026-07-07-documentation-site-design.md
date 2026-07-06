# MailRelay Documentation Site Design

## Goal

Create a static product landing page plus documentation site for MailRelay and deploy it with GitHub Pages. The Nub-inspired landing page is the primary visual deliverable; the Fumadocs-inspired docs layout supports deeper reference material.

## Visual Direction

- Warm off-white canvas inspired by Nub (`#faf7f0`) with deep brown-black text and muted sandstone borders.
- Nub-style landing hero with a narrow sticky navigation, editorial 48px headline, compact install panel, and offset black terminal preview.
- Landing sections present the protocol flow, handler families, security guarantees, and operational commands as spacious editorial stories rather than a generic card grid.
- Compact system sans typography with 28px/600 document titles and 24px/600 section headings.
- Fumadocs-style three-column layout is reserved for `/docs`: persistent left navigation, focused article column, right table of contents.
- Black terminal/code surfaces with restrained syntax color and 8px radius.
- Small coral accent for active navigation and important callouts.
- On mobile, replace side columns with a sticky top bar and one-column article layout.

## Information Architecture

- Introduction: product model, core flow, five-minute setup.
- Getting Started: installation, first configuration, first command.
- Core Concepts: command protocol, Discovery/Catalog, security model, SQLite durability.
- Handlers: HTTP/Webhook, Workflow/Queue, Plugin/Shell, Agent/MCP.
- Operations: CLI, configuration reference, reliability/recovery, deployment and 72-hour soak.

The source of truth remains the current Go implementation and `mailrelay.example.yaml`. Documentation must not promise functionality absent from the repository.

## Technology and Delivery

Use Astro Starlight to generate static HTML under `docs-site/dist`. Custom CSS and component overrides establish the MailRelay visual language. A GitHub Actions workflow builds with pnpm and deploys the generated artifact to Pages. The Astro `base` path derives from `GITHUB_REPOSITORY` during CI so project pages work without hard-coded repository ownership.

## Acceptance

- Static build succeeds locally.
- Every navigation route renders and internal links resolve.
- Desktop and 390px mobile screenshots have no clipping or horizontal overflow.
- Source-vs-implementation QA passes for typography, spacing, colors, code surfaces, and responsive structure.
- Existing Go tests continue to pass.
