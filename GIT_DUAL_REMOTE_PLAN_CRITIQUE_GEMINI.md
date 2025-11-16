# Analysis of `git-dual-remote` Implementation Plan

This document provides an implementation-focused critique of the `git-dual-remote` plan. The provided plan is exceptionally detailed and well-structured, having already incorporated feedback. This analysis focuses on identifying remaining practical challenges, edge cases, and offering concrete solutions to further harden the design.

## 1. Practical Considerations: Hard vs. Straightforward

### Straightforward to Implement
- **CLI Scaffolding**: Using `cobra` for commands and flags is a standard, low-effort task in Go.
- **Basic Git & Shell Operations**: Wrapping `git` commands or using `go-git` for reading configuration is well-understood. The hybrid approach (using both) is practical.
- **UI/UX**: The chosen libraries for terminal output (`zap`, `fatih/color`, `spinner`, `tablewriter`) are mature and will make achieving the desired polished look relatively easy.
- **Pre-flight Checks**: Verifying the presence of dependencies like `git`, `gh`, and `vault` is a simple matter of checking the system `PATH`.

### Hard to Implement
- **Atomic Dual-Push Simulation**: The core challenge is that `git push` is not atomic across multiple remotes. The plan's solution—tracking sync state and providing retry mechanisms—is complex. Reliability hinges on capturing every push event, which is difficult if the user bypasses the tool and uses `git push` directly. The state management logic must be flawless to avoid a confusing user experience.
- **Robust Vault Integration**: The "Pure Vault Configuration" model is clean but operationally demanding. Every command execution requires multiple network roundtrips to Vault before any work is done. The logic for authenticating, fetching, and parsing three separate Vault secrets (tool config, repo SSH key, repo PAT) must be paired with exceptionally clear, actionable error messages for every possible failure mode (network down, token expired, path incorrect, permissions denied).
- **Idempotency & Rollback**: While the plan specifies these goals, the implementation is non-trivial. True transactional changes to a Git config are not possible. A rollback mechanism that relies on restoring a backup (`.git/config.backup`) is a good start, but automatic rollback on any failure is complex to orchestrate. Idempotency requires careful read-before-write logic for every configuration item the tool touches.

## 2. Edge Cases & Corner Cases

- **SSH `known_hosts`**: The tool will fail on its first connection to a new host (either the bare repo server or GitHub with a new key) due to interactive SSH host key verification prompts. This breaks the "fully automated" promise.
- **Pre-existing `core.sshCommand`**: The plan to use a repository-local `core.sshCommand` is excellent for safety. However, it doesn't account for a pre-existing value. Overwriting it could break a user's custom workflow.
- **Complex Git History**: The plan focuses on a simple commit/push workflow. It doesn't address how the tool would handle `git rebase` or `git commit --amend` operations on commits that are already pushed to one remote but not the other. This would require a force push, which the tool needs to manage or guide the user through safely.
- **Git Worktrees**: The logic for finding the `.git` directory should use `git rev-parse --git-dir` to ensure it works correctly in complex setups like Git worktrees, where `.git` might be a file, not a directory.
- **`gh` CLI Authentication State**: The `gh` CLI maintains its own state in `~/.config/gh/`. The tool's plan to authenticate by passing a token could conflict with or be confusing relative to the user's existing `gh` login.
- **Tool "Divorce"**: If the `git-dual-remote` and `gitsetup` tools remain separate, there is no defined process for what happens when `gitsetup` archives or deletes a repository that `git-dual-remote` is configured to manage. The tool's state will become stale and its commands will fail.

## 3. Implementation Details

- **State Management**: The plan proposes a state file (`~/.git-dual-remote/state.yaml`) for tracking sync failures. This file is external to the repository, which means it can be lost, orphaned, or become out of sync if the repository is moved or deleted.
- **Sync Status Verification**: The plan to parse `git ls-remote` is fragile. A more robust method is to compare commit hashes directly using `git rev-parse <remote>/<branch>` for each remote. To calculate ahead/behind counts, the canonical command is `git rev-list --count --left-right <branch>...<remote>/<branch>`.
- **GitHub PAT Handling**: The plan correctly identifies that a PAT is needed for the API, but suggests setting the `GH_TOKEN` environment variable. This is a potential security risk, as the variable could be exposed to subprocesses or persist after a crash.

## 4. Dependencies & Integration

- **The `gitsetup` Decision**: The document's most significant open question is whether to integrate with `gitsetup`. This decision impacts code reuse, user experience, and maintenance. The "Hybrid Approach A: Monorepo with Separate Binaries" is the most pragmatic solution, offering code reuse via a shared `internal` directory without bloating the original `gitsetup` tool or coupling their release cycles.
- **`gh` CLI Versioning**: The tool's dependency on the `gh` CLI is a point of future fragility. `gh` command syntax or output could change, breaking the tool. The pre-flight check should validate not just the presence of `gh`, but a minimum compatible version.

## 5. Testing Strategy

The proposed testing strategy is solid. To make it "bulletproof," the following should be emphasized:

- **Automate against Live Services**: Integration tests should be scripted to run against a temporary, live infrastructure: a `vault server -dev` instance, a local bare Git repository, and a temporary GitHub repository created and destroyed for the test run.
- **"Broken State" Test Suite**: A dedicated test suite should be created to validate the `doctor` and `status` commands. This involves programmatically creating broken configurations (e.g., deleting a remote, mangling an SSH key, creating divergence) and asserting that the tool correctly identifies the issues and recommends the right fixes.
- **Partial Failure Simulation**: The critical dual-push error handling must be tested. This can be done by creating a mock Git remote server that can be configured to fail pushes, allowing the test to verify that the tool correctly detects the partial failure and updates its state.

## 6. Concrete Issues & Solutions

1.  **Issue**: The `post-push` hook, which is the most reliable way to detect remote divergence, is optional.
    *   **Solution**: The `setup` command should **install the `post-push` hook by default**. This makes the tool's safety features proactive. Provide a `--no-hook` flag for users who need to opt out.

2.  **Issue**: The external state file (`~/.git-dual-remote/state.yaml`) is fragile and not bound to the repository.
    *   **Solution**: **Store the tool's state within the repository's own git config** under a custom `[dual-remote]` section. This ensures the state travels with the repo and leverages git's atomic and robust config management.
        ```ini
        # In .git/config
        [dual-remote]
            github-needs-retry = true
            github-last-error = "network unreachable"
        ```

3.  **Issue**: The tool will hang on interactive SSH `known_hosts` prompts.
    *   **Solution**: Automate host key management. During `setup`, fetch the host keys for both remotes using `ssh-keyscan` and store them in a repository-local file (e.g., `.git/dual_remote_known_hosts`). Then, update the `core.sshCommand` to use this file: `ssh -o UserKnownHostsFile=$(pwd)/.git/dual_remote_known_hosts ...`. This is secure, non-interactive, and maintains isolation.

4.  **Issue**: The "Pure Vault Config" makes every command slow and dependent on network access.
    *   **Solution**: Implement a **local file-based cache for the global Vault configuration**. On command execution, use the cached config if it's recent (e.g., <1 hour old), otherwise fetch from Vault. This makes read-only commands like `status` instantaneous and offline-capable while still treating Vault as the single source of truth.

5.  **Issue**: Passing the GitHub PAT via `GH_TOKEN` environment variable is a security risk.
    *   **Solution**: As briefly mentioned in the plan, **always pass the PAT to the `gh` CLI process via `stdin`**. This is the most secure method, as the token never exists in the process environment or on disk.
        ```go
        cmd := exec.Command("gh", "auth", "login", "--with-token")
        cmd.Stdin = strings.NewReader(patFromVault)
        err := cmd.Run()
        ```

By addressing these points, the `git-dual-remote` tool can move from a well-designed plan to a truly robust and "bulletproof" implementation.
