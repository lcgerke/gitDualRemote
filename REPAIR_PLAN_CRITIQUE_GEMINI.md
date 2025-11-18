# Analysis of `githelper repair` Implementation Plan

This document provides an implementation-focused critique of the `githelper repair` command plan. It highlights practical challenges, potential edge cases, and concrete solutions to improve the robustness and safety of the proposed feature.

---

## 1. Practical Considerations

### What's Straightforward?
- **Command Scaffolding**: Creating `cmd/githelper/repair.go` and registering it with Cobra is a standard, low-risk task.
- **Reusing Core Logic**: Leveraging the existing `scenarios.NewClassifier` and `scenarios.SuggestFixes` is efficient and reduces the surface area for new bugs.
- **Filtering Logic**: The functions `filterAutoFixable` and `applyFilters` are pure, easily testable, and simple to implement correctly.
- **Dry Run Mode**: The `--dry-run` flag is a simple conditional check that adds significant safety for users.

### What's Hard to Implement?
- **Interactive Prompts**: Using `fmt.Scanln` for user input is notoriously fragile. It can break when commands are piped, behave unexpectedly across different terminal emulators, and mishandle user input that isn't a simple string followed by a newline. The proposed logic also defaults to "yes" for any input that isn't explicitly "n" or "no", which is a risky default.
- **Error Recovery and State Management**: The plan states it will stop on the first error in auto mode but asks to continue in interactive mode. While this is a good design, ensuring the repository is not left in a broken or intermediate state after a failed operation is very difficult. Git operations are not atomic, and a failed `pull` or `push` can leave things in a state that requires manual intervention.
- **Lack of True Rollback**: The plan correctly identifies that rollback is out of scope. However, this is a major practical consideration. Since `git` commands can be destructive (e.g., a force-push suggested by a future fix), the lack of an undo mechanism makes every operation high-stakes.

---

## 2. Edge Cases & Corner Cases

- **Conflicting Fixes**: The plan assumes all auto-fixable issues are independent. What if two fixes conflict? For example, a fix to stash changes and a fix to pull from a remote. The order of operations matters, but the plan doesn't specify a guaranteed execution order, relying on the order from `SuggestFixes`.
- **Race Conditions**: The workflow is `detect -> fix -> verify`. If another process (or the user in another terminal) modifies the repository state *during* the `fix` step, the `verify` step might report a confusing or incorrect summary.
- **Uninitialized or Empty Repositories**: The plan assumes a fully-functional Git repository. It's unclear how `classifier.Detect()` would behave on a directory with no `.git` folder or an empty one. The command should fail gracefully with a clear error.
- **Hardcoded Remote Name**: The code has a `TODO` for the hardcoded "origin" remote name. This is a significant limitation, as many workflows use different remote names (e.g., "upstream"). In its current state, the `repair` command would fail for those users.
- **`--fix` and `--skip` Overlap**: If a user specifies the same scenario ID in both `--fix` and `--skip` (e.g., `githelper repair --fix S2 --skip S2`), the behavior should be predictable. The current `applyFilters` implementation would correctly skip it, but this interaction should be explicitly documented.

---

## 3. Implementation Details

- **Input Handling**: As mentioned, `fmt.Scanln` is not robust. A dedicated library for interactive prompts would be a significant improvement.
- **Error Handling Contradiction**: The "Decision Points" section says to stop on the first error in auto mode. However, the `runRepair` code snippet shows a loop that continues after an error unless the user interactively chooses to stop. The implementation must be updated to match the stated design decision for `--auto` mode.
- **Auto-Fixing Untracked Files**: The example output shows `[W5] Untracked files present` as an auto-fixable issue with the command `git add <files>`. Automatically adding untracked files is a **highly destructive and presumptive action**. It could add sensitive files, build artifacts, or large binaries to the index. This scenario should **not** be auto-fixable.

---

## 4. Dependencies & Integration

- **`AutoFixExecutor` Brittleness**: The entire safety of the `repair` command rests on the `AutoFixExecutor`. Any bugs, missing preconditions, or destructive operations in the executor will be exposed directly by the `repair` command. The executor must be exceptionally robust.
- **Git Version Compatibility**: The plan does not mention what versions of `git` are supported. If the `AutoFixExecutor` uses modern `git` commands or flags, it could fail on systems with older `git` clients.
- **Fix Order Dependency**: The order of fixes is not guaranteed. This can lead to unpredictable behavior. For example, a `git pull` should only be attempted if the working tree is clean. If a "clean working tree" fix and a "pull remote changes" fix are both pending, the "clean" fix must run first.

---

## 5. Testing Strategy

- **Unit Testing**: The plan for testing the pure filter functions is excellent. However, the main `runRepair` function is difficult to unit test because it directly instantiates its dependencies (`git.NewClient`, `scenarios.NewClassifier`).
- **Integration Testing**: The plan outlines a series of manual `bash` commands. These are a good starting point but should be scripted into an automated test suite. The tests should create temporary git repositories, run the `repair` command, and assert the final state of the repository.
- **Mocking Dependencies**: To properly unit-test the orchestration logic within `runRepair` (e.g., "does it stop on error in auto mode?"), its dependencies should be injectable interfaces. This would allow mock implementations to be passed in during tests.
- **Testing Interactive Input**: Manually testing CLI prompts is unreliable. This can be automated by replacing `os.Stdin` with an in-memory buffer (`bytes.Buffer`) during tests to simulate user input.

---

## 6. Concrete Issues & Solutions

### Issue 1: Fragile and Risky Interactive Prompts
- **Problem**: `fmt.Scanln` is unreliable, and the default-to-yes logic is unsafe.
- **Solution**:
    1.  **Immediate Fix**: Change the prompt logic to default to "no". Any input other than `y` or `yes` (case-insensitive) should be treated as a "no".
    2.  **Recommended Fix**: Replace `fmt.Scanln` with a dedicated CLI prompt library like `github.com/AlecAivazis/survey` for a more robust and user-friendly experience.

### Issue 2: Destructive "Auto-Fix" for Untracked Files
- **Problem**: The `W5` scenario (untracked files) should not be auto-fixable via `git add`. This is a dangerous default.
- **Solution**: Mark `W5` as `AutoFixable: false`. The purpose of `repair` should be to fix repository *state* and *sync issues*, not to make opinionated decisions about a user's code.

### Issue 3: Untestable Orchestration Logic
- **Problem**: The `runRepair` function is tightly coupled to concrete implementations, making it hard to unit-test.
- **Solution**: Use Dependency Injection.
    1.  Define interfaces for dependencies (e.g., `type GitClient interface { ... }`, `type StateClassifier interface { ... }`).
    2.  Refactor `runRepair` and its helper functions to accept these interfaces.
    3.  In `main`, provide the real implementations. In tests, provide mock implementations to simulate various scenarios and errors.

### Issue 4: Unpredictable Fix Execution Order
- **Problem**: The order of fixes is not guaranteed, which can cause operations to fail or have unintended side effects.
- **Solution**:
    1.  **Define a Canonical Order**: Before executing, sort the `fixes` slice by `ScenarioID`. This ensures a deterministic order.
    2.  **Prioritize by Category**: Implement a priority system. For example, working tree fixes (`W*`) should almost always run before synchronization fixes (`S*`).

### Issue 5: Error Handling in Auto Mode
- **Problem**: The sample code does not stop on the first error when in `--auto` mode, contradicting the design.
- **Solution**: Add a check within the execution loop:
    ```go
    // Inside the loop over fixes...
    err := executor.Execute(fix, state)
    if err != nil {
        // ... log error ...
        if autoMode {
            out.Error("Stopping due to error in --auto mode.")
            break // Exit loop immediately
        }
        // ... otherwise, prompt user to continue ...
    }
    ```
