# GitHelper Corpus Validator

Comprehensive testing framework for validating the GitHelper scenario classification system against 100+ real-world repositories.

## Overview

This tool validates that the scenario classifier correctly identifies repository states across diverse codebases. It tests against three types of repositories:

1. **Public repos** (50) - Open source projects with diverse characteristics
2. **Managed repos** (30) - Your production repositories using gitDualRemote
3. **Synthetic repos** (20) - Programmatically generated edge cases

## Quick Start

### Prerequisites

- Go 1.21+
- Git 2.30+
- SSH access to your Core git server (for managed repos)
- ~5GB disk space for cache (optional but recommended)

### Setup

1. **Copy the template:**
   ```bash
   cd test/corpus
   cp repos.yaml.template repos.yaml
   ```

2. **Add your managed repositories:**
   Edit `repos.yaml` and replace the placeholder managed repos with your actual repositories:
   ```yaml
   - name: "myproject-backend"
     url: "git@core-server.example.com:team/backend.git"
     type: "managed"
     notes: "Production backend service"
     tags: ["production", "backend"]
   ```

3. **Configure SSH access:**
   Ensure your SSH keys are set up for your Core git server:
   ```bash
   # Test SSH access
   ssh -T git@your-core-server.example.com

   # If needed, add key to agent
   ssh-add ~/.ssh/id_rsa
   ```

### Running Tests

#### Spike 1: Before Implementation (Baseline)

Run this **before** implementing the classifier to capture current state:

```bash
cd test/corpus
go run . --repos repos.yaml --output spike1.json --html spike1.html
```

This will:
- Clone/cache all repositories
- Run placeholder classifier (returns stub data)
- Generate baseline report
- Create `spike1.json` and `spike1.html`

**Expected outcome:** All repos should return stub scenarios (E1/S1/W1/C1)

#### Spike 2: After Implementation (Validation)

Run this **after** implementing the classifier:

```bash
cd test/corpus
go run . --repos repos.yaml --output spike2.json --html spike2.html --compare spike1.json
```

This will:
- Use cached repos (fast)
- Run real classifier
- Compare against baseline
- Validate against expected scenarios (if provided)

**Success criteria:**
- ✓ False positive rate < 5%
- ✓ No crashes or panics
- ✓ 90% of repos detected in < 2s
- ✓ All expected scenarios match (if defined)

## Configuration

### repos.yaml Format

```yaml
version: "1.0"
description: "Test suite description"
cache_enabled: true
cache_ttl: "7d"  # 7 days, "24h", "30m"
max_concurrent: 5

repos:
  - name: "unique-repo-name"
    url: "git@server:org/repo.git"  # or https://...
    type: "public|managed|synthetic"
    expected:  # Optional - for validation
      existence: E1
      sync: S1
      working_tree: W1
      corruption: C1
      lfs_enabled: false
    notes: "Description"
    tags: ["tag1", "tag2"]
    skip: false
    skip_reason: ""
```

### Expected Scenarios

If you know what scenarios a repo should have, add them to `expected`:

```yaml
- name: "synced-production-repo"
  url: "git@server:team/prod.git"
  type: "managed"
  expected:
    existence: E1    # Local + Core + GitHub all exist
    sync: S1         # All in perfect sync
    working_tree: W1 # Clean working tree
    corruption: C1   # No corruption
    lfs_enabled: false
  notes: "Should always be in sync"
```

The validator will flag mismatches as false positives.

## CLI Reference

### Basic Usage

```bash
# Run with default repos.yaml
go run .

# Specify repos file
go run . --repos custom-repos.yaml

# Generate reports
go run . --output results.json --html results.html

# Compare with baseline
go run . --output spike2.json --compare spike1.json
```

### Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--repos` | Path to repos.yaml | `--repos repos.yaml` |
| `--output` | JSON report file | `--output spike1.json` |
| `--html` | HTML report file | `--html report.html` |
| `--compare` | Compare with baseline | `--compare spike1.json` |
| `--clear-cache` | Clear cache before run | `--clear-cache` |
| `--clear-expired` | Clear expired cache | `--clear-expired` |
| `--cache-stats` | Show cache stats | `--cache-stats` |

### Examples

```bash
# Quick test on subset
go run . --repos small-set.yaml

# Full validation with comparison
go run . --output spike2.json --html spike2.html --compare spike1.json

# Fresh clone (no cache)
go run . --clear-cache --output fresh.json

# Check cache usage
go run . --cache-stats

# Clear old entries
go run . --clear-expired
```

## Cache Management

The validator caches cloned repos to speed up repeated runs.

**Cache location:** `~/.cache/githelper-corpus/`

**Cache behavior:**
- Fresh clone on first access
- `git fetch` update on subsequent runs (if within TTL)
- Automatic expiration after TTL (default: 7 days)

**Manual cache management:**

```bash
# View cache stats
go run . --cache-stats

# Clear expired entries
go run . --clear-expired

# Clear entire cache
go run . --clear-cache

# Disable cache (use temp dirs)
# Edit repos.yaml: cache_enabled: false
```

## Report Formats

### JSON Report

Machine-readable format for automation:

```json
{
  "version": "1.0",
  "generated_at": "2025-11-18T10:00:00Z",
  "git_version": "git version 2.34.1",
  "summary": {
    "total_repos": 100,
    "success_count": 98,
    "failure_count": 2,
    "false_positive_rate": 3.2,
    "avg_detection_time_ms": 1234
  },
  "results": [ ... ],
  "failures": [ ... ],
  "mismatches": [ ... ]
}
```

### HTML Report

Human-readable dashboard with:
- Summary statistics
- Performance metrics
- Failures table
- Mismatches table
- All results table

Open in browser: `open spike1.html`

### Console Summary

Printed to stdout:

```
============== CORPUS TEST SUMMARY ==============
Total Repositories: 100
  ✓ Success:        98
  ✗ Failures:       2
  ⊘ Skipped:        0

Validation (repos with expected scenarios):
  ✓ Matches:        45
  ✗ Mismatches:     3
  ⚠ False Positives: 3 (3.16%)

Performance:
  Avg Detection:    1234 ms
  Max Detection:    2456 ms
  Total Duration:   5m32s

Top Scenario IDs:
  E1: 95
  S1: 78
  W1: 92
  C1: 88

SUCCESS CRITERIA:
  ✓ False positive rate < 5%
  ✗ No crashes or panics
  ✓ 90% of repos detected in < 2s
```

## Adding Managed Repos

### Step 1: Identify Repos

Select 30 production repositories that cover various states:

- ✓ Synced repos (should be S1)
- ✓ Repos with local changes (W2-W5)
- ✓ Repos ahead of remotes (S2, S4)
- ✓ Repos behind remotes (S5, S6, S7)
- ✓ Diverged repos (S8-S13)
- ✓ Repos with LFS (if applicable)

### Step 2: Add to repos.yaml

```yaml
- name: "backend-api"
  url: "git@core.example.com:team/backend.git"
  type: "managed"
  tags: ["production", "golang", "api"]

- name: "frontend-web"
  url: "git@core.example.com:team/frontend.git"
  type: "managed"
  tags: ["production", "react", "web"]

# Add 28 more...
```

### Step 3: Add Expected Scenarios (Optional)

After Spike 1, review results and add expected scenarios for repos where you know the correct state:

```yaml
- name: "backend-api"
  url: "git@core.example.com:team/backend.git"
  type: "managed"
  expected:  # Add after reviewing Spike 1 results
    existence: E1
    sync: S1
    working_tree: W1
    corruption: C1
  tags: ["production", "golang", "api"]
```

## Creating Synthetic Repos

Synthetic repos test specific edge cases. Create these **after** Phase 1 implementation.

### Example: Diverged History

```bash
# Create test repo
cd /tmp
git init diverged-test
cd diverged-test

# Create diverged history
git checkout -b main
echo "base" > file.txt
git add . && git commit -m "base"

# Create remote simulation
git clone --bare . /tmp/diverged-remote.git
git remote add origin /tmp/diverged-remote.git

# Diverge local (ahead 10 commits)
for i in {1..10}; do
  echo "local $i" >> local.txt
  git add . && git commit -m "local commit $i"
done

# Diverge remote (different 10 commits)
cd /tmp/diverged-remote.git
# (use git update-ref to create diverged commits)

# Add to repos.yaml
# - name: "synthetic-diverged"
#   url: "file:///tmp/diverged-remote.git"
#   type: "synthetic"
#   expected:
#     existence: E1
#     sync: S13  # Diverged
#     working_tree: W1
#     corruption: C1
```

### Synthetic Repo Checklist

Create repos covering:

- [ ] Orphan branches
- [ ] Extreme divergence (50+ commits each direction)
- [ ] Large binaries (100MB+)
- [ ] Git LFS repos
- [ ] Shallow clones
- [ ] Many branches (100+)
- [ ] Detached HEAD
- [ ] Corrupted objects
- [ ] Empty repo
- [ ] Single commit repo

(Detailed scripts to be added after Phase 1)

## Troubleshooting

### Clone Failures

**Symptom:** `Clone failed: git clone failed`

**Solutions:**
```bash
# Test SSH access
ssh -T git@your-server

# Add SSH key
ssh-add ~/.ssh/id_rsa

# Use SSH config
cat >> ~/.ssh/config <<EOF
Host core-server
  HostName git.example.com
  User git
  IdentityFile ~/.ssh/id_rsa
EOF

# Update repos.yaml with SSH alias
url: "git@core-server:team/repo.git"
```

### Network Timeouts

**Symptom:** Slow/timeout on large repos

**Solutions:**
- Increase git timeout: `git config --global http.lowSpeedLimit 1000`
- Skip large repos: add `skip: true` in repos.yaml
- Use `--clear-cache` if cache is corrupted

### False Positives

**Symptom:** High false positive rate (>5%)

**Actions:**
1. Review mismatches in HTML report
2. Check if expected scenarios are correct
3. Investigate classifier logic for those scenarios
4. Update expected scenarios if they were wrong
5. File bug report if classifier is incorrect

### Memory Issues

**Symptom:** OOM errors on large corpus

**Solutions:**
- Reduce `max_concurrent` in repos.yaml
- Skip huge repos (linux kernel, chromium)
- Run in batches:
  ```bash
  # Create subset files
  go run . --repos batch1.yaml
  go run . --repos batch2.yaml
  ```

## Integration with CI

### GitHub Actions Example

```yaml
name: Corpus Validation

on:
  pull_request:
    paths:
      - 'internal/scenarios/**'

jobs:
  corpus-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Setup SSH
        uses: webfactory/ssh-agent@v0.8.0
        with:
          ssh-private-key: ${{ secrets.CORE_GIT_SSH_KEY }}

      - name: Run corpus validation
        run: |
          cd test/corpus
          go run . --output results.json --html results.html

      - name: Upload reports
        uses: actions/upload-artifact@v3
        with:
          name: corpus-reports
          path: |
            test/corpus/results.json
            test/corpus/results.html

      - name: Check success criteria
        run: |
          # Fails if false positive rate >= 5%
          # (exit code 1 from validator)
```

## Performance Benchmarks

Expected performance on reference hardware (M1 Mac / 16GB RAM):

| Phase | Repos | Duration | Notes |
|-------|-------|----------|-------|
| Spike 1 (cold) | 70 | ~45 min | Includes cloning all repos |
| Spike 1 (warm) | 70 | ~8 min | Using cache (fetch only) |
| Spike 2 (warm) | 100 | ~10 min | All cached, real classifier |

**Bottlenecks:**
- Cloning linux kernel: ~5 min
- Cloning kubernetes: ~2 min
- Large binary scan: <5s per repo

**Optimization tips:**
- Enable cache: `cache_enabled: true`
- Skip huge repos: mark as `skip: true`
- Increase concurrency: `max_concurrent: 10` (if network/CPU allows)

## Development Workflow

### Two-Spike Strategy

**Goal:** Baseline before implementation, validate after

```bash
# BEFORE implementing classifier (Spike 1)
cd test/corpus
cp repos.yaml.template repos.yaml
# Edit repos.yaml - add your 30 managed repos
go run . --output spike1.json --html spike1.html

# Review spike1.html
# - Verify all repos clone successfully
# - Note any failures (fix SSH, URLs, etc.)
# - Check stub scenarios (all should be E1/S1/W1/C1)
# - Identify repos for expected scenarios
# - Commit spike1.json to git

git add spike1.json repos.yaml
git commit -m "Add corpus baseline (Spike 1)"

# IMPLEMENT classifier (Phases 1-4)
# ... 4-5 weeks of implementation ...

# AFTER implementing classifier (Spike 2)
cd test/corpus
go run . --output spike2.json --html spike2.html --compare spike1.json

# Review spike2.html and comparison
# - Check false positive rate < 5%
# - Review mismatches
# - Fix any bugs found
# - Update expected scenarios

git add spike2.json
git commit -m "Add corpus validation results (Spike 2)"
```

### Adding Expected Scenarios

After Spike 1, manually review results and add expectations:

```bash
# 1. Open spike1.html
# 2. For repos where you KNOW the correct state, add to repos.yaml:

- name: "production-backend"
  url: "git@core:team/backend.git"
  type: "managed"
  expected:  # <-- ADD THIS based on spike1 results
    existence: E1
    sync: S1
    working_tree: W1
    corruption: C1
```

**Target:** Define expected scenarios for 15-20 repos (golden set)

## File Structure

```
test/corpus/
├── README.md              # This file
├── repos.yaml.template    # Template with 50 public repos
├── repos.yaml             # Your copy (gitignored)
├── main.go                # CLI entry point
├── validator.go           # Orchestration logic
├── cache.go               # Repository caching
├── reporter.go            # Report generation
├── types.go               # Data structures
├── spike1.json            # Baseline results (committed)
├── spike2.json            # Validation results (committed)
└── *.html                 # Generated reports (gitignored)
```

## Next Steps

1. ✅ **Setup** - Copy template, add managed repos
2. ✅ **Spike 1** - Run baseline before implementation
3. ⏳ **Implementation** - Build classifier (Phases 1-4)
4. ⏳ **Spike 2** - Validate after implementation
5. ⏳ **Iteration** - Fix issues, re-run tests
6. ⏳ **Ship** - Merge when criteria met

## FAQ

**Q: Do I need all 100 repos?**
A: No. Start with 20-30 (10 public + your managed). Expand as needed. The 100 target is for comprehensive validation.

**Q: Can I use HTTPS instead of SSH?**
A: Yes, but you'll need Git credential helper configured. SSH is recommended for managed repos.

**Q: What if I don't have 30 managed repos?**
A: Use fewer. The principle is to test against YOUR real repos. Even 10 is valuable.

**Q: How do I debug classifier issues?**
A: Check the JSON report for specific repo results. Run classifier manually on problem repos.

**Q: Can I run this in parallel?**
A: Yes, `max_concurrent` controls parallelism. Default is 5. Increase if your network/CPU can handle it.

**Q: What's the cache location?**
A: `~/.cache/githelper-corpus/`. Clear with `--clear-cache`.

## Support

- Issues: https://github.com/lcgerke/gitDualRemote/issues
- Docs: See IMPLEMENTATION_PLAN_v4.md
- Contact: Check repository owner

---

**Version:** 1.0
**Last Updated:** 2025-11-18
**Related:** IMPLEMENTATION_PLAN_v4.md Section 5.3 (Corpus Testing)
