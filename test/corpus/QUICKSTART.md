# Corpus Validator - Quick Start

## ⚡ TL;DR - Run Spike 1 Now

```bash
cd test/corpus
make setup           # Creates repos.yaml from template
# Edit repos.yaml - add your 30 managed repos
make spike1          # Run baseline test (~45 min first time)
# Review spike1.html in browser
git add spike1.json repos.yaml
git commit -m "Add corpus baseline (Spike 1)"
```

Then come back after implementation is done for Spike 2.

---

## 📋 What You Need To Do

### Step 1: Setup (5 minutes)

```bash
cd test/corpus
make setup
```

This creates `repos.yaml` from the template.

### Step 2: Add Your Repos (10 minutes)

Edit `repos.yaml` and add ~30 of your managed repositories:

```yaml
# Find this section in repos.yaml and uncomment/edit:

  - name: "backend-api"
    url: "git@YOUR-CORE-SERVER:team/backend.git"
    type: "managed"
    tags: ["production", "golang"]

  - name: "frontend-web"
    url: "git@YOUR-CORE-SERVER:team/frontend.git"
    type: "managed"
    tags: ["production", "react"]

  # Add 28 more...
```

**Tips:**
- Use actual SSH URLs to your Core git server
- Include repos in different states (synced, diverged, dirty, etc.)
- Don't worry about `expected` scenarios yet (add after Spike 1)

### Step 3: Test SSH Access (2 minutes)

```bash
# Verify you can access your Core server
ssh -T git@your-core-server.example.com

# If needed, add SSH key
ssh-add ~/.ssh/id_rsa
```

### Step 4: Run Spike 1 (45 minutes first time, 8 minutes cached)

```bash
make spike1
```

This will:
- Clone all 70 repos (50 public + 20 yours)
- Run placeholder classifier (returns stub scenarios)
- Generate `spike1.json` and `spike1.html`

**What to expect:**
- First run: ~45 min (cloning everything)
- Subsequent runs: ~8 min (using cache)
- All scenarios will be E1/S1/W1/C1 (stub data)

### Step 5: Review Results (10 minutes)

```bash
open spike1.html
```

**Check for:**
- ✅ All repos cloned successfully
- ✅ No SSH/authentication failures
- ✅ All scenarios detected (even if stub data)

**Fix any issues:**
- Clone failures → Check SSH, URLs
- Timeouts → Skip large repos, increase timeout
- Permission denied → Check SSH keys

### Step 6: Commit Baseline (1 minute)

```bash
git add spike1.json repos.yaml
git commit -m "Add corpus test baseline (Spike 1)

- Added 30 managed repos to test suite
- Baseline results before classifier implementation
- All repos clone successfully"
```

---

## ✅ You're Done With Spike 1!

**What you have now:**
- ✅ 70 test repositories defined
- ✅ Baseline results captured
- ✅ Framework ready for validation

**What happens next:**
1. Claude implements the classifier (Phases 1-4, ~4-5 weeks)
2. You run Spike 2 to validate: `make compare`
3. Review results, fix bugs, iterate

---

## 🔮 Preview: Spike 2 (After Implementation)

After classifier is implemented, you'll run:

```bash
make compare
```

This will:
- Use cached repos (fast!)
- Run REAL classifier
- Compare with spike1.json baseline
- Check success criteria

**Success Criteria:**
- ✓ False positive rate < 5%
- ✓ No crashes or panics
- ✓ 90% of repos detected in < 2s

---

## 🆘 Need Help?

**Common Issues:**

| Problem | Solution |
|---------|----------|
| Can't clone repo | Check SSH: `ssh -T git@server` |
| Permission denied | Add SSH key: `ssh-add ~/.ssh/id_rsa` |
| Timeout on large repo | Add `skip: true` in repos.yaml |
| Don't have 30 repos | Start with 10-15, expand later |

**Resources:**
- Full docs: [README.md](README.md)
- Implementation plan: [IMPLEMENTATION_PLAN_v4.md](../../IMPLEMENTATION_PLAN_v4.md)
- Makefile help: `make help`

---

## 📊 What Spike 1 Does

**Current state** (before classifier):
```
Validator → Clone repos → Run stub classifier → Generate report
                                   ↓
                            Returns E1/S1/W1/C1 for all
```

**Purpose:**
1. Verify all repos are accessible
2. Establish baseline (for comparison later)
3. Identify repos for golden set (expected scenarios)

**After implementation**, the real classifier will replace stubs.

---

## 🚀 Advanced: Custom Repos File

```bash
# Create a small test set
cp repos.yaml test-small.yaml
# Edit to include only 10 repos

# Run on small set
go run . --repos test-small.yaml --output test.json
```

---

## 📝 Summary

1. ✅ `make setup` - Create repos.yaml
2. ✅ Edit repos.yaml - Add your repos
3. ✅ `make spike1` - Run baseline
4. ✅ Review spike1.html
5. ✅ Commit spike1.json
6. ⏳ Wait for implementation
7. ⏳ `make compare` - Validate

**Time investment:**
- Setup: 15-20 minutes
- Spike 1: 45 minutes (first run)
- Review: 10 minutes
- **Total: ~70 minutes**

Then you're ready to validate the full implementation!

---

**Questions?** See [README.md](README.md) for full details.
