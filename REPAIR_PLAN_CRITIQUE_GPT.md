# Repair Plan Critique

## Assumptions

1. **Reusing the existing classifier/suggester/executor stack keeps the repair command safe.** Lines 88‑99 show the plan plans to wire the new CLI straight into `scenarios.NewClassifier`, `SuggestFixes` and `NewAutoFixExecutor`. That is valid: it avoids re‑implementing fix rules and keeps the repair behavior aligned with the status command.
2. **Default detection remote/host names are acceptable.** The implementation sketch hard‑codes `"origin"` and `"github"` for the classifier parameters (REPAIR_PLAN.md:163‑169). This is risky because many repos use non‑standard remote names (e.g., `upstream`) or hosts other than GitHub. The plan calls it out as a TODO but never commits to a concrete flag/lookup so the shipped command would silently skip valid remotes.
3. **One detection run up front is enough for the whole repair.** The command reuses the single `state` that is passed to every `AutoFixExecutor.Execute` call (REPAIR_PLAN.md:176‑237) rather than refreshing it after each fix. That assumes fixes either do not change the prerequisites for later fixes or that the executor ignores the stale state; both are risky when fixes mutate the repo (push/pull/checkout) and later commands might depend on the new sync state.
4. **Interactive mode as the default is the safest user experience.** Mentioned in the Decision Points (REPAIR_PLAN.md:765‑776), this is a defensible assumption for a brand new command because it prevents surprises.

## Gaps

- **Configuration knobs stay TODO.** The plan notes the remote/host should be configurable (REPAIR_PLAN.md:163‑169) but never adds CLI flags or a clear approach to pick the active remote, which means real projects with multiple remotes must rely on conventions that may not hold.
- **Filter scope and validation are underspecified.** `applyFilters` only understands one `--fix` value and a comma list of skips (REPAIR_PLAN.md:287‑315). It never explains how unknown IDs are reported, whether `--fix` can combine with `--skip`, or if multiple `--fix` invocations should be supported to create a subset.
- **State mutation between fixes is unaddressed.** The plan skips any re‑calculation of the repository state until after all fixes run (REPAIR_PLAN.md:260‑269). If a fix pushes commits, the subsequent fix list and prompts should be recalibrated, otherwise the plan may try to apply obsolete operations or continue prompting for fixes that the repository no longer needs.
- **No explicit prompt/input handling strategy.** The interactive loop uses `fmt.Scanln` (REPAIR_PLAN.md:214‑227) but does not describe how it behaves when stdin is piped, how it recognizes an explicit uppercase `N`, or what happens when the user interrupts the process. These details matter for automation and for users running repair inside scripts.

## Technical Feasibility

Overall the implementation is feasible because it composes established components (REPAIR_PLAN.md:88‑99) and adds a straightforward CLI. The biggest technical challenge is ensuring that sequential fixes do not interfere, which the plan currently leaves to the existing executor without verifying (REPAIR_PLAN.md:232‑257). Building the command, prompts, and summary should be standard Cobra/Go work; integration tests against temporary git repos (REPAIR_PLAN.md:438‑465) are practical but will require careful fixture setup.

## Implementation Risks

- **Stale state per fix.** Because `state` is captured before any fixes run and never updated, later fixes may operate on outdated `WorkingTree` or `Sync` values, which is particularly dangerous for fixes that push/pull or clean the working tree (REPAIR_PLAN.md:164‑237).
- **Unclear recovery from partial failures.** The plan stops on error in auto mode or prompts in interactive mode (REPAIR_PLAN.md:240‑249) but does not describe how to unwind a sequence of operations or detect partially applied fixes, so the repo may require manual repairs after a failure.
- **Prompt blocking may break automation.** Interactive prompts rely on blocking `fmt.Scanln` calls, which do not time out and will hang CI if the user forgets to answer or scripts do not supply stdin (REPAIR_PLAN.md:214‑227). There is also no explicit handling for `Ctrl+C` or signals.
- **Integration tests assume network access at `origin`.** The described tests (REPAIR_PLAN.md:437‑465) push/pull against the default remote but do not capture how to mock/seed network access, so running them in a disconnected CI environment could be brittle.

## Timeline Realism

The 4‑hour estimate (REPAIR_PLAN.md:718‑730) looks optimistic. Phase 1 only addresses the core logic, but wiring the CLI, filters, summary, prompts, and error handling takes more than an hour once you account for discovery that `AutoFixExecutor` needs more state, plus updating `cmd/githelper/main.go`. The integration tests alone require crafting multiple temporary repositories with divergent states and verifying pushes/pulls, which is likely to take multiple hours unless there is existing automation. If the timeline is strict, it should explicitly reserve time for troubleshooting flaky git state and for writing the missing configuration hooks.

## Concrete Improvements

1. **Add explicit remote/host selection flags and fallback logic.** Turn the TODO at REPAIR_PLAN.md:163‑169 into flags like `--remote`, `--host`, or a lookup from git config so the repair command works on forks, non‑GitHub remotes, and repos where the useful remote is not named `origin`.
2. **Refresh repository state between fixes.** After each successful fix, rerun `classifier.Detect()` or at least update the `state` with the fix’s `Operation` result before moving to the next fix. That avoids executing stale operations and means the description shown to the user stays correct.
3. **Clarify filter semantics and validation.** Extend `applyFilters` (REPAIR_PLAN.md:287‑315) to support multiple `--fix` IDs, reject unknown IDs earlier, and document how `--skip` works together with `--fix`. This will prevent surprise behavior when a user requests fixes that no longer exist or misspells a scenario ID.
4. **Harden interactive prompts and automation paths.** Describe how prompts behave with piped stdin, allow configuring a default answer (e.g., default “yes” when auto mode is implied), and ensure signal handling doesn’t leave the repo in a half‑repaired state (REPAIR_PLAN.md:214‑249). Consider adding a `--force` flag for scripting.
5. **Expand testing scope.** The unit tests only cover filter helpers (REPAIR_PLAN.md:395‑432). Add tests for `runRepair` using mocked classifiers/ executors to ensure prompts, errors, and summary logic behave correctly, and flesh out the integration scripts to record what remote they expect so they can be rerun in isolated environments without network dependence.
