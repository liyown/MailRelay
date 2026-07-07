# Workflow and Queue Stabilization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote Workflow and Queue from Beta to Stable by making their command graph valid before startup, bounding nested execution, defining safe queue parameter rules, and making terminal failures observable and recoverable.

**Architecture:** Configuration validation owns the static command graph and rejects missing targets, cycles, incompatible queue schemas, and persisted secrets before the runtime starts. Runtime guards remain as defense in depth: Workflow carries an invocation trace and deterministic step IDs, while Queue separates retryable dependency failures from terminal command failures and records sanitized events.

**Tech Stack:** Go 1.25, SQLite, existing `command.Handler`/`command.Executor` interfaces, Fumadocs/MDX documentation.

## Global Constraints

- Do not add a new Handler type or turn Workflow into a general DAG engine.
- Workflow and Queue may invoke only commands declared in the same configuration.
- Queue must not persist executable secret values; v0.1 rejects sensitive Queue schemas instead of adding encryption.
- Persisted errors and runtime events contain fixed safe classifications, never raw provider output or sensitive parameters.
- Existing HTTP/Webhook behavior, reply Outbox semantics, and experimental opt-in behavior remain unchanged.

---

### Task 1: Validate the Workflow and Queue command graph

**Files:**
- Modify: `internal/config/config.go:173`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `validateCommandGraph(commands []command.Command) error`
- Produces: `commandTargets(c command.Command) ([]string, error)`
- Consumes: Workflow `config.steps[].command` and Queue `config.command`.

- [ ] **Step 1: Write failing graph-validation tests**

Add table cases to `internal/config/config_test.go` that construct a valid base `Config`, then call `Validate()` and assert these errors:

```go
func TestValidateRejectsInvalidCommandGraphs(t *testing.T) {
	base := func(commands ...command.Command) Config {
		return Config{Security: Security{Token: "secret", Allow: []string{"me@example.com"}}, Commands: commands}
	}
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"workflow missing target", base(command.Command{Name: "release", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "missing"}}}}), "workflow target missing"},
		{"queue missing target", base(command.Command{Name: "later", Handler: "queue", Config: map[string]any{"command": "missing"}}), "queue target missing"},
		{"indirect workflow cycle", base(
			command.Command{Name: "a", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "b"}}}},
			command.Command{Name: "b", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "a"}}}},
		), "command cycle"},
		{"mixed queue workflow cycle", base(
			command.Command{Name: "a", Handler: "queue", Config: map[string]any{"command": "b"}},
			command.Command{Name: "b", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "a"}}}},
		), "command cycle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error=%v, want %q", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/config -run TestValidateRejectsInvalidCommandGraphs -count=1 -v`

Expected: FAIL because missing targets and indirect cycles are currently accepted.

- [ ] **Step 3: Implement strict target parsing and cycle detection**

In `Config.Validate`, first build `byName map[string]command.Command`, then call `validateCommandGraph(c.Commands)`. `commandTargets` must reject malformed/empty Workflow steps and empty Queue targets. Use DFS colors (`0` unseen, `1` visiting, `2` complete); an edge to a visiting node returns `command cycle involving <name>`. A target absent from `byName` returns `<handler> target <name> is not declared`.

```go
func validateCommandGraph(commands []command.Command) error {
	byName := make(map[string]command.Command, len(commands))
	for _, c := range commands { byName[c.Name] = c }
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 { return fmt.Errorf("command cycle involving %s", name) }
		if state[name] == 2 { return nil }
		state[name] = 1
		targets, err := commandTargets(byName[name])
		if err != nil { return err }
		for _, target := range targets {
			if _, ok := byName[target]; !ok { return fmt.Errorf("%s target %s is not declared", byName[name].Handler, target) }
			if err := visit(target); err != nil { return err }
		}
		state[name] = 2
		return nil
	}
	for name := range byName { if err := visit(name); err != nil { return err } }
	return nil
}
```

- [ ] **Step 4: Verify GREEN and the package suite**

Run: `go test ./internal/config -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "fix: validate workflow and queue command graph"
```

### Task 2: Enforce Queue's stable parameter contract

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/handler/queue_test.go`

**Interfaces:**
- Produces: `validateQueueSchema(wrapper, target command.Command) error`
- Contract: each Queue parameter exists on the target with the same type; every required target parameter is required by the wrapper; neither side marks a queued parameter sensitive.

- [ ] **Step 1: Write failing schema tests**

Add table tests proving `Validate()` rejects: a Queue wrapper parameter absent from the target, mismatched types, a required target parameter omitted by the wrapper, and a sensitive parameter on either wrapper or target. Also add one valid compatible schema case.

```go
wrapper := command.Command{Name: "later", Handler: "queue", Parameters: map[string]command.Parameter{
	"env": {Type: "string", Required: true},
}, Config: map[string]any{"command": "deploy"}}
target := command.Command{Name: "deploy", Handler: "http", Parameters: map[string]command.Parameter{
	"env": {Type: "string", Required: true},
}, Config: map[string]any{"url": "https://api.example.com/deploy"}}
```

Expected sensitive-case error: `queue command later cannot persist sensitive parameter token`.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/config -run TestValidateQueueSchema -count=1 -v`

Expected: FAIL because Queue schemas are not compared today.

- [ ] **Step 3: Implement schema compatibility checks**

Call `validateQueueSchema` after graph targets are known. Normalize empty parameter types to `string`. Reject sensitive declarations before comparing required/type rules. Do not add encryption or silently redact executable values.

- [ ] **Step 4: Replace obsolete Queue secret-persistence expectations**

Remove handler tests that claim a queued worker can execute a redacted secret. Keep a store-boundary regression test using a non-sensitive payload and retain the audit redaction tests for non-Queue handlers. This is a deliberate contract correction: invalid Queue secret configurations never reach `Queue.Execute`.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/config ./internal/handler -count=1`

Expected: PASS with the safe rejection contract covered.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/handler/queue_test.go
git commit -m "fix: define safe queue parameter contract"
```

### Task 3: Bound nested Workflow execution at runtime

**Files:**
- Modify: `internal/command/model.go`
- Modify: `internal/router/router.go`
- Modify: `internal/router/router_test.go`
- Modify: `internal/handler/workflow.go`
- Modify: `internal/handler/workflow_test.go`

**Interfaces:**
- Adds: `command.Request.Trace []string`
- Workflow child ID format: `<parent-message-id>:step:<1-based-index>:<command>`
- Runtime errors: `*command.Error{Kind: "policy", Message: "workflow recursion denied"}` and `*command.Error{Kind: "policy", Message: "workflow depth exceeded"}`.

- [ ] **Step 1: Write failing recursion, depth, and ID tests**

Create a registry/router test with `a -> b -> a` and assert a `policy` error without waiting for context timeout. Expand `workflow_test.go` with repeated targets and assert captured IDs are distinct:

```go
wantIDs := []string{"mail-1:step:1:build", "mail-1:step:2:build"}
```

Add a nested chain longer than `NewWorkflow(64, 2)` and assert `workflow depth exceeded`.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/router ./internal/handler -run 'TestWorkflowIndirectRecursion|TestWorkflowDepth|TestWorkflowStepIDs' -count=1 -v`

Expected: FAIL; `maxDepth` is unused and repeated target names currently collide.

- [ ] **Step 3: Implement trace propagation and safe workflow errors**

Before Router dispatch, reject `req.Name` already present in `req.Trace`. In Workflow, reject when `len(x.Request.Trace)+1 >= w.maxDepth`, then construct the child request with a copied trace plus the current command and an index-based message ID. Wrap step failures without embedding raw dependency text:

```go
return command.Result{}, &command.Error{
	Kind: "workflow_step",
	Message: fmt.Sprintf("step %d (%s) failed", i+1, name),
	Err: err,
}
```

The top-level audit persists `workflow_step`; raw nested errors remain outside SQLite.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/command ./internal/router ./internal/handler -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/command/model.go internal/router/router.go internal/router/router_test.go internal/handler/workflow.go internal/handler/workflow_test.go
git commit -m "fix: bound nested workflow execution"
```

### Task 4: Make Queue failure transitions deterministic and observable

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/handler/queue.go`
- Modify: `internal/handler/queue_test.go`

**Interfaces:**
- Adds: `Store.RejectJob(ctx context.Context, job *Job, kind string) error`
- Terminal kinds: `unknown_command`, `invalid_parameters`, `policy`.
- Retryable kinds: `dependency`, `internal`, and untyped errors.
- Runtime event: phase `queue`, handler `queue`, safe summary `queue job failed`.

- [ ] **Step 1: Write failing terminal/retry/event tests**

Add tests showing an `unknown_command` error moves a claimed job directly to `dead` after one attempt, while a `dependency` error returns it to `pending` with backoff. Assert `runtime_events` contains only the fixed safe summary and classification, even when the executor error contains an email address or token.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/store ./internal/handler -run 'TestRejectJob|TestQueueTerminalFailure|TestQueueRetryableFailure|TestQueueFailureEventIsSafe' -count=1 -v`

Expected: FAIL because every executor error currently follows the same retry path and Queue emits no event.

- [ ] **Step 3: Implement the smallest transition split**

`RejectJob` must update only a currently running row, set `status='dead'`, clear the lease, and persist a fixed allowlisted kind. `RunOneJobWithPolicy` uses `errors.As(err, *command.Error)` to classify terminal errors, calls `RejectJob` for terminal kinds, otherwise calls existing `FailJob`, then writes:

```go
store.RuntimeEvent{
	Severity: "error", Phase: "queue", MessageID: fmt.Sprintf("queue:%d", j.ID),
	Command: j.Command, Handler: "queue", ErrorKind: kind, Summary: "queue job failed",
}
```

If persisting the state transition fails, return that durability error. Event-write failure must also be returned so observability cannot silently diverge from job state.

- [ ] **Step 4: Verify GREEN and lease regressions**

Run: `go test ./internal/store ./internal/handler -count=1`

Expected: PASS, including expired-lease and replay tests.

- [ ] **Step 5: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go internal/handler/queue.go internal/handler/queue_test.go
git commit -m "fix: classify and observe queue failures"
```

### Task 5: Verify end-to-end restart, idempotency, and recovery behavior

**Files:**
- Modify: `internal/app/runtime_test.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: existing `Runtime.Once`, queue lease/replay APIs, and CLI `status`/`replay queue`.
- Produces no new production API.

- [ ] **Step 1: Add an end-to-end Queue lifecycle test**

Build a runtime with a Queue wrapper and deterministic capture target. Prove duplicate wrapper execution returns the same queue row, restart recovers an expired lease, terminal failure appears in `status`, `replay queue <id>` resets it, and successful processing executes the target exactly once after replay.

- [ ] **Step 2: Add an end-to-end Workflow failure test**

Execute a two-step Workflow where step two fails. Assert step one runs once, step two's safe index/name appears in the reply/audit classification, no later step runs, and the outer message reaches the normal reply lifecycle.

- [ ] **Step 3: Run the integration verification**

Run: `go test ./internal/app ./internal/cli -run 'TestQueueRestartReplayLifecycle|TestWorkflowPartialFailureLifecycle' -count=1 -v`

Expected: PASS because Tasks 1-4 already introduced each behavior through focused RED/GREEN cycles. These tests verify composition rather than introduce production behavior.

- [ ] **Step 4: If composition fails, return to a focused RED/GREEN cycle**

Add a minimal focused failing test in the package that owns the broken invariant, run it to confirm RED, make the smallest wiring or classification correction, then rerun the integration test. Do not introduce new commands or storage concepts.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/app ./internal/cli -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/runtime_test.go internal/cli/cli_test.go
git commit -m "test: cover workflow and queue recovery lifecycle"
```

### Task 6: Graduate Workflow and Queue in product surfaces

**Files:**
- Modify: `internal/command/model.go:73`
- Modify: `internal/command/model_test.go`
- Modify: `internal/router/router_test.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `docs-site/content/docs/handlers/workflow-queue.mdx`
- Modify: `docs-site/content/docs/handlers/index.mdx`
- Modify: `docs-site/content/docs/operations/reliability.mdx`
- Modify: `docs-site/scripts/verify-content.mjs`

**Interfaces:**
- `command.HandlerMaturity("workflow") == "Stable"`
- `command.HandlerMaturity("queue") == "Stable"`
- `doctor` emits no maturity warning for Workflow or Queue.

- [ ] **Step 1: Write failing maturity and documentation checks**

Update Go expectations to require Stable and extend `verify-content.mjs` to require the Queue sensitive-parameter prohibition, terminal-vs-retryable behavior, replay procedure, Workflow cycle/depth behavior, and Stable label.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/command ./internal/router ./internal/cli -count=1 && cd docs-site && pnpm check`

Expected: FAIL until labels and docs are updated.

- [ ] **Step 3: Update maturity labels and operator documentation**

Move `workflow` and `queue` into the Stable switch case. Document exact configuration contracts, deterministic stop-on-first-failure Workflow semantics, Queue retry/dead/replay behavior, the ban on persisted sensitive Queue parameters, and recovery commands. Keep Plugin/Shell/Agent/MCP Experimental.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/command ./internal/router ./internal/cli -count=1 && cd docs-site && pnpm check && pnpm build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/command internal/router internal/cli README.md docs-site/content/docs docs-site/scripts/verify-content.mjs
git commit -m "docs: graduate workflow and queue to stable"
```

### Task 7: Completion verification

**Files:**
- Verify: all tracked Go, MDX, TypeScript, JavaScript, and build configuration files.

**Interfaces:**
- Produces: evidence that Phase One meets the Stable gate.

- [ ] **Step 1: Format and run focused tests**

Run: `gofmt -w internal/command internal/config internal/router internal/handler internal/store internal/app internal/cli`

Run: `go test ./internal/config ./internal/router ./internal/handler ./internal/store ./internal/app ./internal/cli -count=1`

- [ ] **Step 2: Run repository-wide checks**

Run:

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/mailrelay-native ./cmd/mailrelay
```

Expected: all exit 0 with no race report or vet diagnostic.

- [ ] **Step 3: Run cross-platform compilation**

Run:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/mailrelay-linux-amd64 ./cmd/mailrelay
GOOS=darwin GOARCH=arm64 go build -o /tmp/mailrelay-darwin-arm64 ./cmd/mailrelay
GOOS=windows GOARCH=amd64 go build -o /tmp/mailrelay-windows-amd64.exe ./cmd/mailrelay
```

Expected: all exit 0.

- [ ] **Step 4: Run docs checks and diff hygiene**

Run: `cd docs-site && pnpm check && pnpm build`

Run: `git diff --check && git status --short`

Expected: checks pass and only intentional files or pre-existing `.superpowers/sdd` scratch files remain.

- [ ] **Step 5: Audit the Stable gate**

Confirm every Workflow/Queue Stable requirement in `docs/superpowers/specs/2026-07-07-product-convergence-and-handler-graduation-design.md` maps to a test or documented contract. Do not change the maturity label if any high-risk requirement remains unproven.

- [ ] **Step 6: Handle verification failures without bundling unrelated fixes**

If a verification command fails, do not create a catch-all commit. Return to the owning task, add a focused failing regression test, implement its minimal correction, rerun Task 7 from Step 1, and commit only the exact test and production files from that focused cycle.
