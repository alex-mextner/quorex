# Run Profiles Model Routing Plan

> For agentic workers: execute one task section per iteration. Keep the PR small
> and additive. This patch is stacked on the parallel external reviewers branch;
> do not rewrite that work.

Goal: add a minimal model-routing foundation through named run profiles that
select existing execution settings as one unit.

## Context

Ralphex already supports these independent settings:

- `claude_command`
- `claude_args`
- `task_model`
- `review_model`
- `external_reviewers`
- `external_review_tool`
- `custom_review_script`

The missing piece is a named profile that can select a coherent set of those
settings without editing the main config for every experiment.

Chosen Claude `-p` compatibility adapter for future recovery work:
`Equality-Machine/claude-p`. Do not implement or recommend `clarp` as the
default path in this patch. `clarp` uses a local proxy and `ANTHROPIC_BASE_URL`;
that requires explicit separate approval before live Claude use.

## Target API

Config:

```ini
run_profile = claude-p-sonnet

[run_profile.claude-p-sonnet]
claude_command = claude-p
claude_args = --dangerously-skip-permissions --output-format stream-json --verbose
task_model = sonnet
review_model = sonnet
external_reviewers = deepseek,codex
```

CLI:

```bash
ralphex --run-profile=claude-p-sonnet docs/plans/feature.md
```

Precedence:

```text
CLI direct flags > CLI --run-profile > selected config run_profile > local config > global config > embedded defaults
```

Example: `--run-profile=claude-p-sonnet --claude-command=claude` should use the
profile settings first, then override only `claude_command` from the direct CLI
flag.

## Non-Goals

- Do not implement parallel coding lanes in this PR.
- Do not implement an "advisor" feature in this PR. Opus/Kimi/GLM/Codex are
  selectable future profiles or pool members, not hidden advisor passes.
- Do not implement provider pools or fallback chains in this PR.
- Do not integrate `umputun/mpt` directly into the coding executor in this PR.
  MPT is a good candidate for a future external review/synthesis script, not
  the first coding executor abstraction.
- Do not auto-merge or compare multiple implementations.
- Do not replace the existing `task_model` / `review_model` semantics.
- Do not change the behavior when no run profile is configured.
- Do not launch Codex CLI from this session. Codex review for this PR must be
  done by a Codex subagent, not by `codex exec`.

## Future Work: Pools and MPT

Do not implement this section in the current PR.

- Review pools should be the next likely increment after run profiles. They can
  add priority, optional reviewers, timeout, and fallback semantics around the
  existing `external_reviewers` mechanism.
- `umputun/mpt` should be evaluated as an external reviewer/synthesizer script:
  it can fan out to DeepSeek, Kimi, GLM, Gemini, OpenAI-compatible providers,
  then return one aggregate review to Ralphex.
- Coding pools are a larger later increment. They need isolated worktrees,
  explicit acceptance suites, and manual winner selection. Do not hide
  parallel coding behind one opaque tool call.

### Task 1: Parse Run Profile Config

- [x] Add `RunProfile` type with fields:
      `Name`, `ClaudeCommand`, `ClaudeArgs`, `TaskModel`, `ReviewModel`,
      `ExternalReviewers`, `ExternalReviewTool`, `CustomReviewScript`.
- [x] Add config values for selected `run_profile` and all
      `[run_profile.<name>]` definitions.
- [x] Add tests in `pkg/config/values_test.go` for:
      selected `run_profile`, multiple profile definitions, no profile,
      and profile fields with `external_reviewers`.
- [x] Run `go test ./pkg/config -run 'RunProfile|ValuesLoader' -count=1`.

### Task 2: Apply Profile With Correct Precedence

- [x] Add profile application so selected config `run_profile` overlays normal
      config values before CLI direct flags are applied.
- [x] Add `--run-profile=<name>` CLI override.
- [x] Ensure CLI direct flags still win after profile application:
      `--claude-command`, `--claude-args`, `--task-model`, `--review-model`,
      `--external-reviewers`, `--external-review-tool`,
      `--custom-review-script`.
- [x] Return a clear error when a selected profile name does not exist.
- [x] Add tests in `pkg/config/config_test.go` and `cmd/ralphex/main_test.go`
      for config-selected profile, CLI-selected profile, missing profile, and
      direct CLI override after profile.
- [x] Run `go test ./pkg/config ./cmd/ralphex -run 'RunProfile|ProviderOverride|Load' -count=1`.

### Task 3: Documentation and Defaults

- [x] Document `run_profile` and `[run_profile.<name>]` in
      `pkg/config/defaults/config`.
- [x] Add README documentation near the provider/model-routing section.
- [x] Include a `claude-p` example using `Equality-Machine/claude-p`.
- [x] State explicitly that `clarp` is not the default recommended adapter
      because it uses a proxy/`ANTHROPIC_BASE_URL` approach.
- [x] Update `CLAUDE.md` with the new config surface and precedence.

### Task 4: Verification

- [ ] Run `make fmt`.
- [ ] Run `go test ./...`.
- [ ] Run `make build`.
- [ ] Run `make lint`.
- [ ] Run `make test`.
- [ ] Confirm `git diff --check` is clean.
- [ ] Summarize the exact API and examples for the PR body.
