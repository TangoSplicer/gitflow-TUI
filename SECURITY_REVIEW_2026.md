# gitflow-TUI Security and Upgrade Review

## Scope

This review treats gitflow-TUI as an interactive GitHub and local-workspace terminal tool. Mutating GitHub actions remain explicit operator actions and are not delegated to the stec evidence platform.

## Improvements applied

| Area | Improvement |
|---|---|
| Subprocess safety | Removed shell-interpolated PR handoff and shell-based CI-log viewing; workflow inputs now reach `gh` as direct arguments. |
| API safety | GitHub search queries use encoded URL parameters, responses have a bounded decode size, non-2xx statuses are surfaced, and the HTTP client has a timeout. |
| Local configuration | Configuration uses the OS config directory, private directory/file permissions, atomic replacement, and safe fallback on malformed JSON. |
| CI | Added read-only GitHub Actions permissions, Go 1.26.x setup, dependency verification, formatting, vet, unit tests, race-test coverage, and builds. |

## Trust boundary

The tool may use the operator's authenticated `gh` session and may open local files or invoke an editor. It must not receive stec case data, local database keys, biometric state, or evidence exports. It should never interpolate repository-derived strings into a shell command.

## Validation

Go 1.26.1 `go test ./...`, `go vet ./...`, `go mod verify`, formatting, and `git diff --check` pass locally. Race testing remains a CI responsibility when the runner provides cgo and the race runtime.
