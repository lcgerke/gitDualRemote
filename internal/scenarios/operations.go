package scenarios

import (
	"context"
	"fmt"

	"github.com/lcgerke/githelper/internal/git"
)

// ============================================================================
// PHASE 1.4: Structured Fix Operations with Safety Validation
// ============================================================================

// FetchOperation - git fetch <remote>
type FetchOperation struct {
	Remote string
}

func (op *FetchOperation) Validate(state *RepositoryState, gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	// Check remote exists and is reachable
	if !gc.CanReachRemote(op.Remote) {
		return fmt.Errorf("remote %s is not reachable", op.Remote)
	}
	return nil
}

func (op *FetchOperation) Execute(gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	return gc.FetchRemote(op.Remote)
}

func (op *FetchOperation) Describe() string {
	return fmt.Sprintf("Fetch updates from %s", op.Remote)
}

func (op *FetchOperation) Rollback(gitClient interface{}) error {
	// Fetch is read-only, no rollback needed
	return nil
}

// PushOperation - git push <remote> <refspec>
type PushOperation struct {
	Remote  string
	Refspec string // e.g., "refs/heads/main" (explicit, not "HEAD")
}

func (op *PushOperation) Validate(state *RepositoryState, gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	// Ensure remote is reachable
	if !gc.CanReachRemote(op.Remote) {
		return fmt.Errorf("remote %s is not reachable", op.Remote)
	}

	// Ensure working tree is clean
	if !state.WorkingTree.Clean {
		return fmt.Errorf("working tree must be clean before push (found %d staged, %d unstaged files)",
			len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
	}

	return nil
}

func (op *PushOperation) Execute(gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	return gc.Push(op.Remote, op.Refspec)
}

func (op *PushOperation) Describe() string {
	return fmt.Sprintf("Push %s to %s", op.Refspec, op.Remote)
}

func (op *PushOperation) Rollback(gitClient interface{}) error {
	// Push cannot be safely rolled back
	return fmt.Errorf("push operations cannot be automatically rolled back")
}

// ResetOperation - git reset --hard <ref> (with CRITICAL fast-forward validation)
type ResetOperation struct {
	Ref string // Full ref: "refs/remotes/origin/main"
}

func (op *ResetOperation) Validate(state *RepositoryState, gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	// Validation 1: Working tree must be clean
	if !state.WorkingTree.Clean {
		return fmt.Errorf("working tree must be clean before reset (found %d staged, %d unstaged files)",
			len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
	}

	// Validation 2: Ref must exist
	if op.Ref == "" {
		return fmt.Errorf("reset ref cannot be empty")
	}

	// CRITICAL: Validation 3: Must be fast-forward only (prevent data loss)
	// Check if target ref is ancestor of current HEAD
	// If HEAD has commits not in target, this would discard them
	isAncestor, err := gc.IsAncestor(op.Ref, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to check fast-forward status: %w", err)
	}

	if !isAncestor {
		return fmt.Errorf("reset would discard local commits (not a fast-forward); target ref is not ancestor of HEAD")
	}

	return nil
}

func (op *ResetOperation) Execute(gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	return gc.ResetToRef(op.Ref)
}

func (op *ResetOperation) Describe() string {
	return fmt.Sprintf("Reset to %s (fast-forward only)", op.Ref)
}

func (op *ResetOperation) Rollback(gitClient interface{}) error {
	// Reset rollback is risky - better to fail
	return fmt.Errorf("reset rollback requires manual intervention (use: git reset --hard ORIG_HEAD)")
}

// PullOperation - equivalent to fetch + reset (safer than git pull)
type PullOperation struct {
	Remote string
	Branch string
}

func (op *PullOperation) Validate(state *RepositoryState, gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	// Ensure remote is reachable
	if !gc.CanReachRemote(op.Remote) {
		return fmt.Errorf("remote %s is not reachable", op.Remote)
	}

	// Ensure working tree is clean
	if !state.WorkingTree.Clean {
		return fmt.Errorf("working tree must be clean before pull (found %d staged, %d unstaged files)",
			len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
	}

	// Check that it will be a fast-forward (after fetch)
	ref := fmt.Sprintf("%s/%s", op.Remote, op.Branch)
	isAncestor, err := gc.IsAncestor(ref, "HEAD")
	if err != nil {
		// Ref might not exist yet (fetch will get it)
		return nil
	}

	if !isAncestor {
		return fmt.Errorf("pull would discard local commits (not a fast-forward)")
	}

	return nil
}

func (op *PullOperation) Execute(gitClient interface{}) error {
	gc, ok := gitClient.(*git.Client)
	if !ok {
		return fmt.Errorf("invalid git client type")
	}

	// Step 1: Fetch
	if err := gc.FetchRemote(op.Remote); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Step 2: Reset to remote branch (safe because we validated fast-forward)
	ref := fmt.Sprintf("%s/%s", op.Remote, op.Branch)
	if err := gc.ResetToRef(ref); err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}

	return nil
}

func (op *PullOperation) Describe() string {
	return fmt.Sprintf("Pull from %s/%s (fetch + reset)", op.Remote, op.Branch)
}

func (op *PullOperation) Rollback(gitClient interface{}) error {
	return fmt.Errorf("pull rollback requires manual intervention (use: git reset --hard ORIG_HEAD)")
}

// CompositeOperation - sequence of operations (executed in order)
type CompositeOperation struct {
	Operations  []Operation
	StopOnError bool // If true, stop at first error
}

func (op *CompositeOperation) Validate(state *RepositoryState, gitClient interface{}) error {
	for _, subOp := range op.Operations {
		if err := subOp.Validate(state, gitClient); err != nil {
			return fmt.Errorf("validation failed for %s: %w", subOp.Describe(), err)
		}
	}
	return nil
}

func (op *CompositeOperation) Execute(gitClient interface{}) error {
	for i, subOp := range op.Operations {
		if err := subOp.Execute(gitClient); err != nil {
			if op.StopOnError {
				return fmt.Errorf("operation %d/%d failed (%s): %w",
					i+1, len(op.Operations), subOp.Describe(), err)
			}
		}
	}
	return nil
}

func (op *CompositeOperation) Describe() string {
	return fmt.Sprintf("Composite operation (%d steps)", len(op.Operations))
}

func (op *CompositeOperation) Rollback(gitClient interface{}) error {
	// Disabled per v4 plan - too risky for multi-step operations
	return fmt.Errorf("composite operation rollback disabled - manual intervention required")
}

// ============================================================================
// Auto-Fix Executor
// ============================================================================

// AutoFixExecutor applies auto-fixable operations with validation
type AutoFixExecutor struct {
	gitClient *git.Client
	ctx       context.Context
}

// NewAutoFixExecutor creates a new auto-fix executor
func NewAutoFixExecutor(gitClient *git.Client) *AutoFixExecutor {
	return &AutoFixExecutor{
		gitClient: gitClient,
		ctx:       context.Background(),
	}
}

// Execute applies a fix with validation
func (afe *AutoFixExecutor) Execute(fix Fix, state *RepositoryState) error {
	if !fix.AutoFixable {
		return fmt.Errorf("fix for %s is not auto-fixable", fix.ScenarioID)
	}

	if fix.Operation == nil {
		return fmt.Errorf("fix for %s has no operation defined", fix.ScenarioID)
	}

	// Validate operation is safe
	if err := fix.Operation.Validate(state, afe.gitClient); err != nil {
		return fmt.Errorf("validation failed for %s: %w", fix.ScenarioID, err)
	}

	// Execute operation
	if err := fix.Operation.Execute(afe.gitClient); err != nil {
		// Attempt rollback on failure (will fail for most operations)
		rollbackErr := fix.Operation.Rollback(afe.gitClient)
		if rollbackErr != nil {
			return fmt.Errorf("execution failed: %w (rollback also failed: %v)", err, rollbackErr)
		}
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}

// ExecuteAll applies multiple fixes in order
func (afe *AutoFixExecutor) ExecuteAll(fixes []Fix, state *RepositoryState) AutoFixResult {
	result := AutoFixResult{
		Applied: []Fix{},
		Failed:  []Fix{},
		Errors:  []error{},
	}

	for _, fix := range fixes {
		if !fix.AutoFixable || fix.Operation == nil {
			continue
		}

		if err := afe.Execute(fix, state); err != nil {
			result.Failed = append(result.Failed, fix)
			result.Errors = append(result.Errors, err)
		} else {
			result.Applied = append(result.Applied, fix)
		}
	}

	return result
}
