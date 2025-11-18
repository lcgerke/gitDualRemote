# Bug Investigation Report for GitHelper v3.0

Based on the implementation plan for `GitHelper v4.0` provided in `input.md`, this report analyzes the critical bugs present in the hypothetical preceding version (`v3.0`). The v4.0 plan is designed to fix these issues.

---

## 1. Critical Bug: Data Loss in Auto-Fix `ResetOperation`

This is the most severe bug, as it can lead to silent, irreversible data loss for the user.

- **Bug Mechanics**: The `repair --auto` command could be used to "fix" a repository where the local branch was behind a remote. The fix involved a `git reset --hard <remote-ref>`. However, this operation did not check if the local branch *also* had unique commits not present on the remote. In a diverged-branch scenario, the hard reset would discard the local-only commits, losing the user's work.

- **Data Flow Analysis**:
    1. A user runs `githelper repair --auto`.
    2. The `Classifier` detects a scenario like `S7` ("Core ahead of local").
    3. The `Suggester` creates a `ResetOperation` targeting the remote branch (e.g., `origin/main`).
    4. The operation's `Validate()` method in v3.0 only checked if the working tree was clean. It **did not** check if the reset was a "fast-forward."
    5. `Execute()` runs `git reset --hard origin/main`, which succeeds even if the local `main` has commits that `origin/main` does not.
    6. The user's local commits are silently deleted.

- **State Management**: The state of the user's local branch is corrupted. Commits that were part of the local branch's history are erased. The `ORIG_HEAD` would be the only immediate way to recover, but this is non-obvious and unreliable.

- **Test Cases**:
    - **Failing Test Case (Catches the bug)**:
        1. Initialize a git repository with a remote `origin`.
        2. Create a commit "A" and push it. `origin/main` and `local/main` are at "A".
        3. Create a local-only commit "B". `local/main` is at "B".
        4. Push a new commit "C" directly to the remote (simulating another developer's work). `origin/main` is at "C". The branches have diverged.
        5. Run the buggy `repair --auto`. The tool incorrectly sees `origin/main` as "ahead" and triggers a reset.
        6. **Expected Bug Behavior**: The local branch is reset to "C", and commit "B" is lost.
    - **Missing Test**: A test that specifically creates a diverged history and asserts that the auto-fix operation *fails* with an error instead of proceeding.

- **Implementation Fix**: The v4.0 plan introduces a fast-forward check using `git merge-base --is-ancestor`. This ensures a reset only happens if it doesn't discard local work.

    ```go
    // In internal/scenarios/operations.go

    // **UPDATED in v4:** ResetOperation with CRITICAL fast-forward validation
    type ResetOperation struct {
        Ref string  // Full ref: "refs/remotes/origin/main"
    }

    func (op *ResetOperation) Validate(ctx context.Context, state *RepositoryState, gc *git.Client) error {
        // Validation 1: Working tree must be clean
        if !state.WorkingTree.Clean {
            return fmt.Errorf("working tree must be clean before reset...")
        }

        // ... other checks ...

        // **CRITICAL in v4:** Validation 3: Must be fast-forward only (prevent data loss)
        // Check if target ref is an ancestor of the current HEAD.
        // If not, it means HEAD has commits that would be discarded.
        isAncestor, err := gc.IsAncestor(op.Ref, "HEAD")
        if err != nil {
            return fmt.Errorf("failed to check fast-forward status: %w", err)
        }

        if !isAncestor {
            return fmt.Errorf("reset would discard local commits (not a fast-forward); target ref is not ancestor of HEAD")
        }

        return nil
    }
    ```

- **Verification**:
    1. Apply the fix from the v4.0 plan.
    2. Run the "Failing Test Case" again.
    3. The `repair` command should now fail before executing the reset, printing the error: "reset would discard local commits (not a fast-forward)". This confirms data loss is prevented.

---

## 2. Bug: Race Conditions on Concurrent Git Operations

This bug leads to flaky, unpredictable behavior and potential repository corruption.

- **Bug Mechanics**: The `git.Client` in v3.0 was not thread-safe. Commands like `doctor` could trigger concurrent `git fetch` operations to multiple remotes (`core` and `github`). Git commands are not designed to run concurrently in the same repository directory; doing so can cause one command to fail because another holds a lock (e.g., on `.git/index.lock`).

- **Data Flow Analysis**:
    1. `doctor` command is initiated.
    2. The system spawns two goroutines to refresh remote data: `git fetch core` and `git fetch github`.
    3. Both commands execute `exec.Command("git", ...)` at nearly the same time.
    4. The first `git fetch` acquires a lock on the repository's object database.
    5. The second `git fetch` attempts to run, but fails because the repository is locked by the first process. This results in an error and potentially incomplete state information for the classifier.

- **State Management**: The repository's internal state becomes unreliable. A failed fetch means the classifier might be operating on stale data. In worse cases, simultaneous writes could lead to object corruption.

- **Test Cases**:
    - **Failing Test Case (Catches the bug)**:
        1. Create a test that spawns 10+ goroutines.
        2. Each goroutine calls `gitClient.FetchRemote("origin")`.
        3. **Expected Bug Behavior**: The test execution will be flaky. Some `fetch` calls will fail with errors like "fatal: unable to create '/path/to/repo/.git/index.lock': File exists."

- **Implementation Fix**: The v4.0 plan introduces a `sync.Mutex` in the `git.Client` to serialize all git command executions, ensuring only one operates at a time.

    ```go
    // In internal/git/cli.go
    
    // Client wraps git operations with thread-safety
    type Client struct {
        repoPath string
        mu       sync.Mutex  // **NEW in v4:** Serialize all git operations
    }

    // **UPDATED in v4:** run() with mutex
    func (c *Client) run(ctx context.Context, args ...string) (string, error) {
        // **CRITICAL:** Serialize all git operations to prevent races
        c.mu.Lock()
        defer c.mu.Unlock()

        cmd := exec.CommandContext(ctx, "git", args...)
        // ... rest of the execution logic
    }
    ```

- **Verification**:
    1. Apply the fix.
    2. Run the multi-goroutine test case again.
    3. All `fetch` calls should now succeed. The operations will execute sequentially, and the test will pass reliably without any lock file errors.

---

## 3. Bug: Indefinite Hang on Interactive Credential Prompts

This bug causes the application to freeze, requiring a manual kill from the user.

- **Bug Mechanics**: When a git command (`fetch`, `push`) requires credentials (e.g., for an HTTPS remote) and a credential helper isn't configured, Git prompts for a username/password on the command line. Since the Go application is not running in an interactive terminal, this prompt is never seen, and the process blocks forever, waiting for input that can never be provided.

- **Data Flow Analysis**:
    1. A user runs `githelper doctor` against a repository with an HTTPS remote URL.
    2. The user has no cached credentials for this remote.
    3. The application calls `git.Client.FetchRemote(...)`.
    4. The underlying `exec.Command("git", "fetch", ...)` call is made.
    5. The Git process waits for a password to be entered on `stdin`.
    6. The `cmd.Run()` call in Go blocks indefinitely.

- **State Management**: The application state is frozen. The goroutine responsible for the fetch is permanently blocked, and the application becomes unresponsive.

- **Test Cases**:
    - **Failing Test Case (Catches the bug)**:
        1. Set up a local web server that serves a git repository and requires basic authentication.
        2. Configure a test repository to use this HTTPS remote URL.
        3. Ensure no git credential helper is configured.
        4. Execute a `FetchOperation`.
        5. **Expected Bug Behavior**: The test runner hangs until it times out.

- **Implementation Fix**: The v4.0 plan adds `GIT_TERMINAL_PROMPT=0` to the environment of every git command. This tells Git to fail immediately rather than prompt for credentials interactively.

    ```go
    // In internal/git/cli.go -> run()

    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = c.repoPath

    // **NEW in v4:** Force non-interactive mode
    cmd.Env = append(os.Environ(),
        "GIT_TERMINAL_PROMPT=0",  // Prevent credential hangs
        "LC_ALL=C",
    )
    ```

- **Verification**:
    1. Apply the fix.
    2. Run the hanging test case again.
    3. The `FetchOperation` should now fail immediately with a "Permission denied" or "Authentication failed" error from Git, and the test will complete without timing out.