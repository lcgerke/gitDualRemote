package scenarios

import (
	"errors"
	"testing"
)

// MockGitClient implements gitClient interface for testing
type MockGitClient struct {
	localHash  string
	coreHash   string
	githubHash string
	ahead      int
	behind     int

	fetchError       error
	branchHashError  error
	remoteHashError  error
	countError       error
}

func (m *MockGitClient) FetchRemote(remote string) error {
	return m.fetchError
}

func (m *MockGitClient) GetBranchHash(branch string) (string, error) {
	if m.branchHashError != nil {
		return "", m.branchHashError
	}
	return m.localHash, nil
}

func (m *MockGitClient) GetRemoteBranchHash(remote, branch string) (string, error) {
	if m.remoteHashError != nil {
		return "", m.remoteHashError
	}
	if remote == "origin" || remote == "core" {
		return m.coreHash, nil
	}
	return m.githubHash, nil
}

func (m *MockGitClient) CountCommitsBetween(ref1, ref2 string) (int, error) {
	if m.countError != nil {
		return 0, m.countError
	}
	// Simple logic for testing: if ref1 is local and ref2 is remote, return ahead
	// if ref1 is remote and ref2 is local, return behind
	if ref1 == m.localHash {
		return m.ahead, nil
	}
	return m.behind, nil
}

// Test E2 scenario: local ahead of core
func TestDetectTwoWaySync_E2_LocalAhead(t *testing.T) {
	mockGC := &MockGitClient{
		localHash: "abc123",
		coreHash:  "def456",
		ahead:     9,
		behind:    0,
	}

	// Create classifier with mock client (cast to *git.Client interface)
	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S2" {
		t.Errorf("Expected S2, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.LocalAheadOfCore != 9 {
		t.Errorf("Expected LocalAheadOfCore=9, got %d", state.LocalAheadOfCore)
	}
	if state.LocalBehindCore != 0 {
		t.Errorf("Expected LocalBehindCore=0, got %d", state.LocalBehindCore)
	}
	if state.AvailableRemote != "origin" {
		t.Errorf("Expected AvailableRemote=origin, got %s", state.AvailableRemote)
	}
}

// Test E2 scenario: local behind core
func TestDetectTwoWaySync_E2_LocalBehind(t *testing.T) {
	mockGC := &MockGitClient{
		localHash: "abc123",
		coreHash:  "xyz789",
		ahead:     0,
		behind:    5,
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S3" {
		t.Errorf("Expected S3, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.LocalAheadOfCore != 0 {
		t.Errorf("Expected LocalAheadOfCore=0, got %d", state.LocalAheadOfCore)
	}
	if state.LocalBehindCore != 5 {
		t.Errorf("Expected LocalBehindCore=5, got %d", state.LocalBehindCore)
	}
}

// Test E2 scenario: in sync
func TestDetectTwoWaySync_E2_InSync(t *testing.T) {
	mockGC := &MockGitClient{
		localHash: "abc123",
		coreHash:  "abc123",
		ahead:     0,
		behind:    0,
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S1" {
		t.Errorf("Expected S1, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.LocalAheadOfCore != 0 {
		t.Errorf("Expected LocalAheadOfCore=0, got %d", state.LocalAheadOfCore)
	}
	if state.LocalBehindCore != 0 {
		t.Errorf("Expected LocalBehindCore=0, got %d", state.LocalBehindCore)
	}
}

// Test E2 scenario: diverged (both ahead and behind)
func TestDetectTwoWaySync_E2_Diverged(t *testing.T) {
	mockGC := &MockGitClient{
		localHash: "abc123",
		coreHash:  "def456",
		ahead:     3,
		behind:    2,
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S4" {
		t.Errorf("Expected S4, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if !state.Diverged {
		t.Error("Expected Diverged to be true")
	}
	if state.LocalAheadOfCore != 3 {
		t.Errorf("Expected LocalAheadOfCore=3, got %d", state.LocalAheadOfCore)
	}
	if state.LocalBehindCore != 2 {
		t.Errorf("Expected LocalBehindCore=2, got %d", state.LocalBehindCore)
	}
}

// Test E2 scenario: remote fetch fails
func TestDetectTwoWaySync_E2_RemoteUnavailable(t *testing.T) {
	mockGC := &MockGitClient{
		fetchError: errors.New("ssh: connect to host core.example.com port 22: Connection refused"),
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S_UNAVAILABLE" {
		t.Errorf("Expected S_UNAVAILABLE, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.Error == "" {
		t.Error("Expected Error to be set")
	}
	if state.Error != "ssh: connect to host core.example.com port 22: Connection refused" {
		t.Errorf("Unexpected error message: %s", state.Error)
	}
}

// Test E2 scenario: detached HEAD
func TestDetectTwoWaySync_E2_DetachedHEAD(t *testing.T) {
	mockGC := &MockGitClient{
		branchHashError: errors.New("not on a branch"),
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S_NA_DETACHED" {
		t.Errorf("Expected S_NA_DETACHED, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.Error == "" {
		t.Error("Expected Error to be set")
	}
}

// Test E2 scenario: remote branch not found
func TestDetectTwoWaySync_E2_RemoteBranchNotFound(t *testing.T) {
	mockGC := &MockGitClient{
		localHash:       "abc123",
		remoteHashError: errors.New("fatal: couldn't find remote ref main"),
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

	if state.ID != "S_UNAVAILABLE" {
		t.Errorf("Expected S_UNAVAILABLE, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
}

// Test E3 scenario: local ahead of github
func TestDetectTwoWaySync_E3_LocalAhead(t *testing.T) {
	mockGC := &MockGitClient{
		localHash:  "abc123",
		githubHash: "def456",
		ahead:      4,
		behind:     0,
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "", "github", "Core")

	if state.ID != "S2" {
		t.Errorf("Expected S2, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.LocalAheadOfGitHub != 4 {
		t.Errorf("Expected LocalAheadOfGitHub=4, got %d", state.LocalAheadOfGitHub)
	}
	if state.LocalBehindGitHub != 0 {
		t.Errorf("Expected LocalBehindGitHub=0, got %d", state.LocalBehindGitHub)
	}
	if state.AvailableRemote != "github" {
		t.Errorf("Expected AvailableRemote=github, got %s", state.AvailableRemote)
	}
}

// Test E3 scenario: local behind github
func TestDetectTwoWaySync_E3_LocalBehind(t *testing.T) {
	mockGC := &MockGitClient{
		localHash:  "abc123",
		githubHash: "xyz789",
		ahead:      0,
		behind:     2,
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "", "github", "Core")

	if state.ID != "S3" {
		t.Errorf("Expected S3, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.LocalAheadOfGitHub != 0 {
		t.Errorf("Expected LocalAheadOfGitHub=0, got %d", state.LocalAheadOfGitHub)
	}
	if state.LocalBehindGitHub != 2 {
		t.Errorf("Expected LocalBehindGitHub=2, got %d", state.LocalBehindGitHub)
	}
}

// Test E3 scenario: remote fetch fails
func TestDetectTwoWaySync_E3_RemoteUnavailable(t *testing.T) {
	mockGC := &MockGitClient{
		fetchError: errors.New("fatal: unable to access 'https://github.com/user/repo.git/': Could not resolve host"),
	}

	classifier := &Classifier{
		gitClient:    mockGC,
		coreRemote:   "origin",
		githubRemote: "github",
		options:      DefaultDetectionOptions(),
	}

	state := classifier.detectTwoWaySync(mockGC, "main", "", "github", "Core")

	if state.ID != "S_UNAVAILABLE" {
		t.Errorf("Expected S_UNAVAILABLE, got %s", state.ID)
	}
	if !state.PartialSync {
		t.Error("Expected PartialSync to be true")
	}
	if state.Error == "" {
		t.Error("Expected Error to be set")
	}
}
