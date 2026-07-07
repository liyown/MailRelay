# MailRelay Web Console Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a production-buildable, authenticated, read-only MailRelay operations console with the selected Warm Operations UI, backed by real Go APIs and embedded in the single MailRelay binary.

**Architecture:** A Vite React SPA under `console/` consumes versioned JSON endpoints from a focused `internal/web` package. Go owns authentication, safe data projection, SQLite queries, and static embedding; the frontend owns presentation, filtering, responsive navigation, and query lifecycle without direct YAML or SQLite access.

**Tech Stack:** Go 1.25, SQLite, `embed`, React, TypeScript, Vite, Tailwind CSS, shadcn/ui, Radix UI, TanStack Query/Table, Recharts, React Hook Form, Zod, Lucide, Vitest, Testing Library, MSW, Playwright.

## Global Constraints

- Phase 1 is read-only except login/logout; replay and configuration writes remain Phase 2/3.
- Default Web address is exactly `127.0.0.1:8787`; non-loopback binding requires explicit configuration.
- The API never returns mailbox passwords, Command tokens, API keys, webhook secrets, MCP credentials, full mail bodies, or raw provider errors.
- The selected visual target is `docs/design/mailrelay-console-warm-operations.png` at desktop viewport `1440x1024`.
- Use the shared Warm Operations tokens and shadcn primitives; do not hand-roll buttons, inputs, dialogs, menus, tables, sheets, tooltips, badges, or skeletons.
- All pages use the same 4px spacing grid, 36px controls, 44px table rows, 8px control radius, 10px panel radius, and Encode Sans Variable typography.
- Existing CLI behavior, docs site, handler runtime, and default disabled WebUI behavior remain unchanged.

---

### Task 1: Scaffold the console and lock the visual foundation

**Files:**
- Create: `console/package.json`
- Create: `console/pnpm-lock.yaml`
- Create: `console/vite.config.ts`
- Create: `console/tsconfig.json`
- Create: `console/tsconfig.app.json`
- Create: `console/index.html`
- Create: `console/components.json`
- Create: `console/src/main.tsx`
- Create: `console/src/app.tsx`
- Create: `console/src/styles/globals.css`
- Create: `console/src/test/setup.ts`
- Create: `console/src/app.test.tsx`
- Create: `console/src/lib/utils.ts`
- Create: `console/src/components/ui/*` through the shadcn CLI
- Reference: `docs/design/mailrelay-console-warm-operations.png`

**Interfaces:**
- Produces: `pnpm dev`, `pnpm test`, `pnpm build`, `pnpm lint`.
- Produces: CSS tokens `--background`, `--foreground`, `--card`, `--border`, `--primary`, `--success`, `--warning`, `--destructive`, `--info`.
- Produces: shared shadcn primitives used by every later task.

- [ ] **Step 1: Scaffold generated framework files**

Run from the repository root:

```bash
pnpm create vite console --template react-ts --no-interactive
cd console
pnpm install
pnpm dlx shadcn@latest init --defaults
pnpm dlx shadcn@latest add button badge card input label select separator sheet skeleton table tabs tooltip dropdown-menu avatar alert
pnpm add @tanstack/react-query @tanstack/react-table react-router-dom recharts react-hook-form @hookform/resolvers zod lucide-react @fontsource-variable/encode-sans
pnpm add -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom msw eslint-plugin-jsx-a11y
```

Expected: `console/` contains a React TypeScript Vite project and generated shadcn components. Generated scaffolding is the only no-test-first step in this task.

- [ ] **Step 2: Write the first failing application test**

```tsx
// console/src/app.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { App } from './app';

describe('App', () => {
  it('renders the MailRelay console identity', () => {
    render(<App />);
    expect(screen.getByText('MailRelay')).toBeInTheDocument();
    expect(screen.getByText('运行控制台')).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run the test and verify RED**

Run: `cd console && pnpm test -- --run`

Expected: FAIL because `App` does not render the selected console identity.

- [ ] **Step 4: Implement the root providers and exact design tokens**

`src/main.tsx` creates `QueryClientProvider`, `BrowserRouter`, and `TooltipProvider`. `src/styles/globals.css` imports Encode Sans and defines:

```css
:root {
  --background: 42 50% 96%;
  --foreground: 24 15% 9%;
  --card: 40 100% 99%;
  --card-foreground: 24 15% 9%;
  --muted: 39 30% 90%;
  --muted-foreground: 31 10% 38%;
  --border: 39 26% 85%;
  --primary: 13 84% 38%;
  --primary-foreground: 40 100% 99%;
  --success: 132 61% 33%;
  --warning: 36 91% 41%;
  --destructive: 5 66% 47%;
  --info: 214 52% 43%;
  --radius: 0.5rem;
}
```

`App` initially renders the identity block only; no dashboard mock data is added in this task.

- [ ] **Step 5: Verify GREEN and production build**

Run: `cd console && pnpm test -- --run && pnpm build`

Expected: PASS and a `console/dist` production bundle.

- [ ] **Step 6: Commit**

```bash
git add console
git commit -m "feat: scaffold web console design system"
```

### Task 2: Add Web configuration and secure authentication primitives

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/web/auth.go`
- Create: `internal/web/auth_test.go`
- Create: `internal/web/errors.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `config.Web` with `Enabled bool`, `Address string`, `PublicURL string`, `SessionSecret string`, `AdminPasswordHash string`, `SessionTTL time.Duration`.
- Produces: `web.SessionManager` with `Login`, `Logout`, `RequireSession`, `RequireCSRF`.
- Produces: JSON error envelope `{"error":{"code","message","fields","request_id"}}`.

- [ ] **Step 1: Write failing Web configuration tests**

Add cases proving defaults and secure validation:

```go
func TestWebDefaultsAndValidation(t *testing.T) {
  c := validConfig()
  c.Web.Enabled = true
  if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "web.session_secret") {
    t.Fatalf("expected missing session secret, got %v", err)
  }
  c.Web.SessionSecret = strings.Repeat("s", 32)
  c.Web.AdminPasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0c2FsdA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
  if err := c.Validate(); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/config -run TestWebDefaultsAndValidation -count=1 -v`

Expected: FAIL because `Config.Web` does not exist.

- [ ] **Step 3: Implement Web config decoding and validation**

Defaults: `Address="127.0.0.1:8787"`, `SessionTTL=8*time.Hour`. When enabled, require at least 32 bytes of `SessionSecret`, a non-empty Argon2id hash, a valid host:port, and an `http` or `https` PublicURL when provided.

- [ ] **Step 4: Write failing authentication tests**

Cover correct/incorrect password, expired cookie, tampered signature, `HttpOnly`, `SameSite=Strict`, logout clearing, and CSRF mismatch. Use a fixed injected clock and deterministic random reader.

```go
func TestSessionRejectsTamperedCookie(t *testing.T) {
  sessions := newTestSessionManager(t)
  cookie, csrf := sessions.issue("admin")
  cookie.Value += "x"
  req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
  req.AddCookie(cookie)
  if sessions.authenticated(req) { t.Fatal("tampered cookie authenticated") }
  if csrf == "" { t.Fatal("missing csrf token") }
}
```

- [ ] **Step 5: Run and verify RED**

Run: `go test ./internal/web -run 'TestSession|TestLogin|TestCSRF' -count=1 -v`

Expected: FAIL because `internal/web` authentication does not exist.

- [ ] **Step 6: Implement Argon2id password verification and signed sessions**

Add `golang.org/x/crypto/argon2`. Session payload is base64url JSON `{sub,exp,csrf}` plus HMAC-SHA256 signature. Use constant-time signature and password comparisons. `POST /login` rotates the cookie; `POST /logout` expires it. Never log the supplied password, cookie, or CSRF token.

- [ ] **Step 7: Verify GREEN**

Run: `go test ./internal/config ./internal/web -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config internal/web go.mod go.sum
git commit -m "feat: add web console authentication"
```

### Task 3: Create safe read models and paginated Store queries

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Create: `internal/web/models.go`
- Create: `internal/web/repository.go`
- Create: `internal/web/repository_test.go`

**Interfaces:**
- Produces: `Dashboard(rangeKey string) (Dashboard, error)`.
- Produces: `Executions(ExecutionFilter) (Page[ExecutionItem], error)`.
- Produces: `Jobs(JobFilter) (Page[JobItem], error)`.
- Produces: `Replies(ReplyFilter) (Page[ReplyItem], error)`.
- Produces: `Events(EventFilter) (Page[EventItem], error)`.
- Produces: `Commands() []CommandItem` and `System() SystemInfo`.
- Cursor contract: base64url-encoded last numeric ID, ordered descending, `limit` clamped to `1..100`.

- [ ] **Step 1: Write failing query and redaction tests**

Seed executions, runtime events, queue jobs, replies, and a sensitive Command. Assert descending stable pagination, filters, KPI aggregation, and no secret fields:

```go
func TestRepositoryNeverReturnsPersistedSecretFields(t *testing.T) {
  repo := seededRepository(t)
  page, err := repo.Executions(context.Background(), ExecutionFilter{Limit: 20})
  if err != nil { t.Fatal(err) }
  raw, _ := json.Marshal(page)
  for _, forbidden := range []string{"topsecret", "mail-password", "api-key"} {
    if bytes.Contains(raw, []byte(forbidden)) { t.Fatalf("leaked %s", forbidden) }
  }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/web ./internal/store -run 'TestRepository|TestExecutionPage|TestDashboard' -count=1 -v`

Expected: FAIL because the read repository and list queries do not exist.

- [ ] **Step 3: Implement focused Store queries**

Add SQL methods that select explicit safe columns. Never use `SELECT *`. Dashboard supports `24h`, `7d`, and `30d`, returning fixed time buckets, totals, success rate, P95 duration, active handler count, health counts, handler distribution, and five recent executions/events.

- [ ] **Step 4: Implement DTO mapping**

`models.go` contains JSON-only DTOs; it must not expose `config.Config`, `store.Store`, raw SQL rows, mail payloads, or secret config maps. Command config is projected to maturity, parameter count, description, and handler only in Phase 1.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/store ./internal/web -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store internal/web/models.go internal/web/repository.go internal/web/repository_test.go
git commit -m "feat: add console read models"
```

### Task 4: Expose the authenticated `/api/v1` read API

**Files:**
- Create: `internal/web/server.go`
- Create: `internal/web/server_test.go`
- Create: `internal/web/routes.go`
- Create: `internal/web/routes_test.go`

**Interfaces:**
- Produces: `web.NewServer(Options) http.Handler`.
- Produces routes: `/api/v1/login`, `/logout`, `/session`, `/dashboard`, `/health`, `/commands`, `/executions`, `/jobs`, `/replies`, `/events`, `/system`.
- All authenticated responses set `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`.

- [ ] **Step 1: Write failing route contract tests**

Use `httptest.Server` and assert unauthenticated 401, malformed filters 400, authenticated JSON shape, request IDs, body limit, method rejection, no-store headers, and absent secret strings.

```go
func TestHealthRequiresSession(t *testing.T) {
  srv := httptest.NewServer(newTestServer(t))
  defer srv.Close()
  res, err := http.Get(srv.URL + "/api/v1/health")
  if err != nil { t.Fatal(err) }
  if res.StatusCode != http.StatusUnauthorized { t.Fatalf("status=%d", res.StatusCode) }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/web -run 'TestHealth|TestDashboardRoute|TestListRoutes|TestSecurityHeaders' -count=1 -v`

Expected: FAIL because the server and routes do not exist.

- [ ] **Step 3: Implement the router and middleware**

Use Go 1.25 `http.ServeMux` method patterns. Middleware order: request ID → recovery → security headers → 1 MiB body limit → authentication → handler. Login is the only unauthenticated JSON route. Map typed validation errors to stable codes and Chinese-safe messages.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/web -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web
git commit -m "feat: expose console read api"
```

### Task 5: Embed the SPA and integrate console startup

**Files:**
- Create: `internal/web/assets.go`
- Create: `internal/web/assets_test.go`
- Create: `internal/web/dist/.gitkeep`
- Create: `internal/web/generate.go`
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `cmd/mailrelay/main.go`
- Modify: `Makefile` or create `Makefile` if absent

**Interfaces:**
- Produces: `web.Start(ctx, address, handler) error` with graceful shutdown.
- Produces: `mailrelay console --config <path>`.
- `mailrelay run` starts Web only when `web.enabled=true`.
- SPA fallback serves `index.html` for non-API GET routes; `/api/*` never falls back.

- [ ] **Step 1: Write failing embed and CLI tests**

Assert `/` returns the SPA, `/assets/app-123.js` returns immutable assets, `/commands` falls back to SPA, `/api/v1/missing` returns JSON 404, disabled Web does not listen, and `console` appears in usage.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/web ./internal/app ./internal/cli -run 'TestEmbedded|TestWebDisabled|TestConsoleCommand' -count=1 -v`

Expected: FAIL because assets and startup integration do not exist.

- [ ] **Step 3: Implement deterministic frontend generation**

Create `internal/web/generate.go`:

```go
//go:generate sh -c "cd ../../console && pnpm install --frozen-lockfile && pnpm build && rm -rf ../internal/web/dist/* && cp -R dist/. ../internal/web/dist/"
package web
```

Embed `dist/*` with `//go:embed`. Return strong ETags for hashed assets and `no-cache` for `index.html`.

- [ ] **Step 4: Integrate lifecycle and graceful shutdown**

Runtime owns an optional `http.Server`. Shutdown uses a five-second context. `console` builds Runtime dependencies and serves Web without starting IMAP polling. Binding errors return synchronously and safely.

- [ ] **Step 5: Verify GREEN**

Run: `go generate ./internal/web && go test ./internal/web ./internal/app ./internal/cli -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/web internal/app internal/cli cmd/mailrelay Makefile
git commit -m "feat: embed and start web console"
```

### Task 6: Build login, application shell, and responsive navigation

**Files:**
- Create: `console/src/api/client.ts`
- Create: `console/src/api/types.ts`
- Create: `console/src/api/session.ts`
- Create: `console/src/routes/login.tsx`
- Create: `console/src/routes/root.tsx`
- Create: `console/src/components/app-sidebar.tsx`
- Create: `console/src/components/app-header.tsx`
- Create: `console/src/components/page-header.tsx`
- Create: `console/src/components/status-badge.tsx`
- Create: `console/src/components/query-state.tsx`
- Create: `console/src/routes/root.test.tsx`
- Create: `console/src/routes/login.test.tsx`

**Interfaces:**
- Produces: `api<T>(path, init): Promise<T>` with credentials, JSON errors, and CSRF header.
- Produces: protected router shell and seven functional read-only routes.
- Mobile navigation uses shadcn `Sheet`; desktop sidebar width is 248px.

- [ ] **Step 1: Write failing login and shell tests**

Test successful login redirect, invalid login field error, unauthenticated redirect, active navigation, sidebar collapse, mobile Sheet, keyboard focus, and logout.

- [ ] **Step 2: Run and verify RED**

Run: `cd console && pnpm test -- --run src/routes/login.test.tsx src/routes/root.test.tsx`

Expected: FAIL because auth routes and shell do not exist.

- [ ] **Step 3: Implement the shared shell from the selected visual**

Use Lucide icons: `LayoutDashboard`, `TerminalSquare`, `History`, `ListRestart`, `ScrollText`, `ShieldCheck`, `Settings`. Header contains environment, search affordance, health status, notifications, help, and avatar menu. Route labels and spacing match the design spec exactly.

- [ ] **Step 4: Implement query states and accessibility**

Every routed page has loading Skeleton, stale banner, error state with retry, and empty state. Sidebar buttons have text alternatives; Sheet returns focus on close; active nav uses both color and `aria-current`.

- [ ] **Step 5: Verify GREEN**

Run: `cd console && pnpm test -- --run && pnpm build`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add console/src
git commit -m "feat: add console login and application shell"
```

### Task 7: Implement the real Warm Operations dashboard

**Files:**
- Create: `console/src/api/dashboard.ts`
- Create: `console/src/routes/dashboard.tsx`
- Create: `console/src/routes/dashboard.test.tsx`
- Create: `console/src/components/metric-block.tsx`
- Create: `console/src/components/activity-chart.tsx`
- Create: `console/src/components/runtime-health.tsx`
- Create: `console/src/components/queue-summary.tsx`
- Create: `console/src/components/recent-executions.tsx`
- Create: `console/src/mocks/handlers.ts`

**Interfaces:**
- Consumes: `GET /api/v1/dashboard?range=24h|7d|30d`.
- Refresh interval: 30 seconds; dashboard data stale after 20 seconds.
- Charts share `#B33210`, success `#218739`, warning `#C87A0A`, error `#C53A2A`.

- [ ] **Step 1: Write failing dashboard behavior tests**

Use MSW fixtures and assert KPI formatting, range changes, chart accessible summary, runtime degraded state, recent execution links, loading, empty, error, and stale states.

- [ ] **Step 2: Run and verify RED**

Run: `cd console && pnpm test -- --run src/routes/dashboard.test.tsx`

Expected: FAIL because the dashboard does not exist.

- [ ] **Step 3: Implement the dashboard without mock-only shortcuts**

All visible values come from the API hook. Recharts uses a responsive container, shared tooltip, fixed time formatting, and an adjacent visually hidden table for screen readers. KPI blocks use dividers rather than nested cards. Runtime health and Queue summaries map directly to API states.

- [ ] **Step 4: Verify GREEN and compare at 1440x1024**

Run: `cd console && pnpm test -- --run src/routes/dashboard.test.tsx && pnpm build`

Expected: PASS. Rendered layout matches `docs/design/mailrelay-console-warm-operations.png` in hierarchy, palette, density, sidebar, header, charts, health summary, and execution table.

- [ ] **Step 5: Commit**

```bash
git add console/src
git commit -m "feat: add operations dashboard"
```

### Task 8: Add real read-only operational pages

**Files:**
- Create: `console/src/api/commands.ts`
- Create: `console/src/api/executions.ts`
- Create: `console/src/api/jobs.ts`
- Create: `console/src/api/events.ts`
- Create: `console/src/api/system.ts`
- Create: `console/src/routes/commands.tsx`
- Create: `console/src/routes/executions.tsx`
- Create: `console/src/routes/queue.tsx`
- Create: `console/src/routes/events.tsx`
- Create: `console/src/routes/mail-security.tsx`
- Create: `console/src/routes/system.tsx`
- Create: `console/src/components/data-table.tsx`
- Create: `console/src/components/filter-bar.tsx`
- Create: `console/src/components/detail-sheet.tsx`
- Create: `console/src/routes/operations.test.tsx`

**Interfaces:**
- Consumes the Phase 1 read APIs with cursor pagination.
- Produces functional filtering, pagination, detail sheets, refresh, URL search params, and responsive column visibility.
- Mail/security and system pages display safe configured state only; no edit controls in Phase 1.

- [ ] **Step 1: Write failing operations-page tests**

Cover server filter parameters, next cursor, detail Sheet, status badge mapping, Queue/Reply/Dead tabs, event auto-refresh, safe config projection, empty/error states, and absence of replay/edit buttons.

- [ ] **Step 2: Run and verify RED**

Run: `cd console && pnpm test -- --run src/routes/operations.test.tsx`

Expected: FAIL because the routes and shared table do not exist.

- [ ] **Step 3: Implement the shared table and filters**

Use TanStack Table for rendering and explicit server-side state; never sort or paginate a partial page as if it were complete. Filter state is encoded in URL search parameters. Detail uses shadcn Sheet and preserves table scroll position.

- [ ] **Step 4: Implement all read-only routes**

Command shows name, handler, maturity, parameter count and description. Execution shows safe fields only. Queue uses Queue/Reply/Dead tabs. Events use severity/phase filters and 10-second refresh. Mail/security and system use definition lists and status badges, not disabled editable forms.

- [ ] **Step 5: Verify GREEN**

Run: `cd console && pnpm test -- --run && pnpm build`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add console/src
git commit -m "feat: add console operations pages"
```

### Task 9: End-to-end, accessibility, visual QA, and release verification

**Files:**
- Create: `console/playwright.config.ts`
- Create: `console/e2e/login.spec.ts`
- Create: `console/e2e/dashboard.spec.ts`
- Create: `console/e2e/operations.spec.ts`
- Create: `console/e2e/accessibility.spec.ts`
- Modify: `.github/workflows/ci.yml`
- Modify: `.goreleaser.yml`
- Modify: `README.md`
- Modify: `docs-site/content/docs/operations/cli.mdx`

**Interfaces:**
- Produces: CI checks for console test/build, Go generation, Go test/race/vet, and embedded asset freshness.
- Produces: native binary containing the console bundle.

- [ ] **Step 1: Write Playwright journeys**

Test login failure/success, dashboard range change, execution filter and detail, Queue tab switching, event refresh, logout, desktop `1440x1024`, tablet `1024x768`, and mobile `390x844`. Add axe scans for login, dashboard, and execution pages with zero serious/critical violations.

- [ ] **Step 2: Run E2E and capture the dashboard**

Run: `cd console && pnpm exec playwright test`

Expected: PASS and a dashboard screenshot at `console/test-results/dashboard-1440.png`.

- [ ] **Step 3: Perform visual comparison**

Compare `console/test-results/dashboard-1440.png` with `docs/design/mailrelay-console-warm-operations.png` at the same viewport. Correct visible hierarchy, spacing, typography, border, radius, density, chart, and responsive issues. Do not chase generated text inaccuracies from the raster reference; preserve product-correct MailRelay labels.

- [ ] **Step 4: Update CI and operator docs**

CI runs `pnpm install --frozen-lockfile`, `pnpm test -- --run`, `pnpm build`, `go generate ./internal/web`, `git diff --exit-code internal/web/dist`, then existing Go checks. README documents `web.enabled`, password hash generation, default loopback address, and `mailrelay console`.

- [ ] **Step 5: Run full verification**

```bash
cd console && pnpm lint && pnpm test -- --run && pnpm build && pnpm exec playwright test
cd ..
go generate ./internal/web
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/mailrelay-console ./cmd/mailrelay
git diff --check
```

Expected: every command exits 0; no generated asset drift, race, vet diagnostic, severe accessibility finding, or unexpected tracked change.

- [ ] **Step 6: Commit**

```bash
git add console .github/workflows/ci.yml .goreleaser.yml README.md docs-site/content/docs/operations/cli.mdx internal/web/dist
git commit -m "feat: complete web console phase one"
```
