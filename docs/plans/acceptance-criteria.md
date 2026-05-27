# Quorex Acceptance Criteria

Adversarial evaluation: goal is to find defects, not confirm correctness.

## Build & Infrastructure

| Criterion | Test | Result |
|-----------|------|--------|
| Binary builds cleanly | `go build ./cmd/quorex/` | PASS |
| Module path is `github.com/alex-mextner/quorex` | `go list -m` | PASS |
| All imports updated (no `umputun/ralphex`) | `grep -r umputun/ralphex .` | PASS |
| .gitignore covers quorex binary + worktrees | manual check | PASS |
| Pre-commit hook runs build+test+lint | `git commit` | PASS |
| Pre-commit hook unsets GIT_INDEX_FILE (prevents test corruption) | hook script | PASS |

## Unit Tests

| Package | Tests | Result |
|---------|-------|--------|
| `pkg/quorexcfg` | 12 | PASS |
| `pkg/hooks` | 6 | PASS |
| `pkg/planval` | 8 (incl. regression) | PASS |
| `pkg/pool` | 5 | PASS |
| `pkg/judge` | 4 | PASS |
| `pkg/initcmd` | 4 | PASS |
| `pkg/executor` | +2 (GenericExecutor) | PASS |
| **Total** | **41 new** | **PASS** |
| Race detector | `go test -race` on all new packages | PASS |

## E2E Tests (CLI)

| Scenario | Command | Result |
|----------|---------|--------|
| Version output | `quorex --version` | PASS — prints "quorex" |
| Valid plan accepted | `quorex plans fix valid.md` | PASS — exit 0 |
| Bad ## heading rejected | `quorex plans fix bad.md` | PASS — exit 1, shows Line N |
| Unclosed fence detected | `quorex plans fix fence.md` | PASS — exit 1, shows line |
| Multiple errors all shown | `quorex plans fix multi.md` | PASS |
| Missing file error | `quorex plans fix /nope` | PASS — exit 1, stderr msg |
| No args to plans | `quorex plans` | PASS — exit 1, usage |
| Fence inside section OK | `quorex plans fix ok_fence.md` | PASS — valid |
| Auto-fix invokes harness | `quorex plans fix --auto bad.md` | PASS — runs fake claude |
| Init no harness | `quorex init` (empty PATH) | PASS — prints install guide |
| Init detects harness | `quorex init` (fake claude in PATH) | PASS — prints harness name |

## Adversarial Testing Results

| Test | Finding | Status |
|------|---------|--------|
| Empty plan file | Returns "valid" (no task sections) | ACCEPTABLE — blank plan is valid |
| Plan with no `##` sections | Returns "valid" | ACCEPTABLE — no tasks required |
| `## Task: ` (empty name) | Was accepted → now rejected | **BUG FIXED** |
| `### subheading` not checked | Not flagged | CORRECT |
| Worktree collision (same executor+runID) | Git rejects with error | CORRECT |
| Pool with 0 executors | Returns `[]`, no error | CORRECT |
| Invalid timeout `"notaduration"` | `Parse()` returns error | CORRECT |
| `ParseMatrix` with garbage input | Returns empty categories | CORRECT (graceful) |
| `ParseMatrix` with unknown provider | Winner marked, not in Providers list | ACCEPTABLE — scores still usable |
| `quorex plans fix` with claude in PATH | Was auto-invoking → now requires `--auto` | **FIXED** |

## Pre-existing Failures (Inherited from ralphex)

| Test | Issue | Decision |
|------|-------|----------|
| `TestService_CreateWorktreeForPlan/fails_when_branch_is_checked_out_in_another_worktree` | Git version returns different error message than test expects | DO NOT FIX — pre-existing, unrelated to quorex changes |

## Acceptance Decision

**ACCEPTED** with known limitations:

1. No pool integration test with live AI executors (requires claude/codex at runtime — not automatable in CI)
2. Judge two-pass is prompt-builder only; actual LLM invocation is via existing ralphex processor (correct by design — judge synthesis uses the same iteration loop)
3. `quorex init` harness detection prints next steps but doesn't generate `quorex.toml` automatically (by design: AI does this step)
4. Telegram notification not wired in (mechanism documented in `~/.claude/AGENTS.md`; implementation would require a bot token)
