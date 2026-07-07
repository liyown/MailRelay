# Task 3 Report: Operational Guardrails, Doctor, and Soak

## Scope

Implemented Task 3 in:

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/app/runtime.go`
- `internal/app/runtime_test.go`
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `README.md`

Also wrote this report at:

- `.superpowers/sdd/task-3-report.md`

## TDD Evidence

### Red phase

Added the required failing tests first:

- `TestExperimentalHandlersRequireOptIn`
- `TestRuntimeRetryDefaults`
- `TestConfigReloadFalseIgnoresFileChanges`
- `TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault`
- updated `TestVersionAndZeroDurationSoak` expectations

Then ran the focused commands from the brief:

```bash
go test ./internal/config -run 'TestExperimentalHandlersRequireOptIn|TestRuntimeRetryDefaults' -count=1 -v
go test ./internal/app -run TestConfigReloadFalseIgnoresFileChanges -count=1 -v
go test ./internal/cli -run 'TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault|TestVersionAndZeroDurationSoak' -count=1 -v
```

Observed failures matched the intended missing behavior:

- config package failed because `Runtime` did not yet expose the new retry/backoff fields
- runtime reload-disabled test failed because `reloadIfChanged` still applied file changes
- doctor labeling test failed because output did not yet separate local and network checks

### Green phase

Implemented:

- runtime config fields for experimental opt-in and retry/backoff defaults
- runtime YAML decode for duration strings and `config_reload` defaulting
- experimental handler validation gate
- reload short-circuit when `runtime.config_reload` is `false`
- doctor output split into `local checks:` and `network checks: skipped`
- soak summary based on `Store.Health`
- README operational guidance updates

## Final verification

Focused verification:

```bash
go test ./internal/config -count=1
go test ./internal/app -run 'TestHotReloadIsAtomicAndKeepsLastValidConfig|TestConfigReloadFalseIgnoresFileChanges' -count=1 -v
go test ./internal/cli -run 'TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault|TestVersionAndZeroDurationSoak|TestDoctorWarnsForExperimentalHandler' -count=1 -v
```

Required package sweep:

```bash
go test ./internal/config ./internal/app ./internal/cli -count=1
```

All of the above passed.

## Behavior delivered

- Experimental handlers now require `runtime.enable_experimental: true`.
- Runtime retry/backoff defaults now load as:
  - `reply_max_attempts = 5`
  - `queue_max_attempts = 3`
  - `initial_backoff = 1m`
  - `max_backoff = 30m`
- Hot reload is now fully disabled when `runtime.config_reload: false`.
- `doctor` now labels local checks clearly and skips active network dialing by default.
- `soak` now reports health-based dead/pending/stale counters and fails on any non-zero invariant breach.

## Notes

- No unrelated files were modified.
- Existing untracked `.superpowers/sdd` task artifacts were left untouched.
