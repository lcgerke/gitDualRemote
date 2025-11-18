# Fix Plan: Enable Codex Data Capture in Sampler

## Problem Statement

After wake-from-sleep, Codex metrics show N/A while Claude metrics display (potentially stale) data. Root cause: **sampler never captures Codex usage data**, only Claude data.

**Current State:**
- ✅ `cmd/codex-status/` tool exists and is fully implemented
- ✅ Analyzer has Codex metric types (`codex_pace_ratio`, `codex_usage_ratio`, `codex_reset_seconds`)
- ✅ Analyzer processes events with `source_tool="codex"` correctly
- ✅ Widget displays Codex dials (C5, CW) and queries Codex metrics
- ❌ **Sampler only calls `cctrack-claude-status`, never `cctrack-codex-status`**
- ❌ **No Codex events ever written to JSONL files**
- ❌ **Codex Prometheus gauges never populated**

## Goals

1. Modify sampler to capture both Claude AND Codex usage data every 5 minutes
2. Write both sources to JSONL files with proper `source_tool` tagging
3. Leverage existing per-source checkpoint logic in analyzer
4. Maintain backward compatibility with existing JSONL files
5. Zero changes needed to analyzer or widget (already support Codex)

## Architecture Design

### Current Data Flow
```
Sampler (every 5min)
    ↓
calls: cctrack-claude-status only
    ↓
writes: {"source_tool":"claude", ...} → usage-YYYYMMDD.jsonl
    ↓
Analyzer reads → processes Claude events → updates claude_* metrics
    ↓
Widget queries → shows Claude data, Codex=N/A
```

### Target Data Flow
```
Sampler (every 5min)
    ↓
calls: cctrack-claude-status AND cctrack-codex-status (parallel)
    ↓
writes: Two events per cycle to usage-YYYYMMDD.jsonl:
        {"source_tool":"claude", "seq_no":N, ...}
        {"source_tool":"codex", "seq_no":N+1, ...}
    ↓
Analyzer reads → processes both sources independently (per-source checkpoint)
    ↓
Widget queries → shows both Claude + Codex live data
```

## Implementation Plan

### Phase 1: Build Infrastructure (15 minutes)

**Task 1.1: Build codex-status binary**
```bash
cd /home/lcgerke/ccTrack
go build -o bin/cctrack-codex-status ./cmd/codex-status
./bin/cctrack-codex-status --version  # Verify build
```

**Task 1.2: Verify codex-status works standalone**
```bash
./bin/cctrack-codex-status --format json
# Expected: JSON output with Codex usage windows (5h_limit, weekly_limit)
```

**Task 1.3: Add codex-status path to sampler config**

File: `cmd/sampler/config.go`

Add field to `CaptureConfig`:
```go
type CaptureConfig struct {
    ClaudeStatusPath string        // Existing
    CodexStatusPath  string         // NEW: Path to cctrack-codex-status
    // ... rest of fields
}
```

Update `DefaultSamplerConfig()`:
```go
func DefaultSamplerConfig() *SamplerConfig {
    return &SamplerConfig{
        // ...
        Capture: CaptureConfig{
            ClaudeStatusPath: "./bin/cctrack-claude-status",
            CodexStatusPath:  "./bin/cctrack-codex-status",  // NEW
            // ...
        },
    }
}
```

Add CLI flag in `cmd/sampler/main.go` parseFlags():
```go
flag.StringVar(&config.Capture.CodexStatusPath, "codex-status",
    getEnv("CODEX_STATUS_PATH", config.Capture.CodexStatusPath),
    "Path to cctrack-codex-status binary")
```

### Phase 2: Dual Capture Logic (30 minutes)

**Task 2.1: Create parallel capture functions**

File: `cmd/sampler/capture.go`

Add new function `runCodexStatus()`:
```go
// runCodexStatus executes codex-status and returns the parsed event
func runCodexStatus(ctx context.Context, config *CaptureConfig) (*UsageEvent, error) {
    logger.Info("runCodexStatus: Starting")

    // Build command (reuse buildClaudeStatusArgs pattern)
    args := []string{
        "--format", "json",
        "--columns", strconv.Itoa(config.Columns),
        "--lines", strconv.Itoa(config.Lines),
        "--status-timeout", "20",
    }

    if config.TmuxSocket != "" {
        args = append(args, "--tmux-socket", config.TmuxSocket)
    }

    cmd := exec.CommandContext(ctx, config.CodexStatusPath, args...)
    logger.Infow("runCodexStatus: Command built", "path", config.CodexStatusPath)

    output, err := cmd.Output()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            logger.Error("runCodexStatus: Command timed out")
            return nil, fmt.Errorf("codex-status timed out after %v", config.Timeout)
        }
        fullCmd := exec.CommandContext(ctx, config.CodexStatusPath, args...)
        combinedOutput, _ := fullCmd.CombinedOutput()
        return nil, fmt.Errorf("codex-status command failed: %s", string(combinedOutput))
    }

    // Parse JSON output
    event, err := parseCodexStatusOutput(output)
    if err != nil {
        return nil, err
    }

    logger.Info("runCodexStatus: Success, returning event")
    return event, nil
}

// parseCodexStatusOutput parses JSON output from codex-status into UsageEvent
func parseCodexStatusOutput(jsonData []byte) (*UsageEvent, error) {
    var event UsageEvent
    if err := json.Unmarshal(jsonData, &event); err != nil {
        return nil, fmt.Errorf("failed to parse codex-status JSON: %w", err)
    }
    return &event, nil
}
```

**Task 2.2: Modify CaptureUsage to capture both sources**

Option A: Sequential capture (simpler, safer):
```go
// CaptureBothSources runs capture for both Claude and Codex
// Returns two events (Claude, Codex) or errors
func CaptureBothSources(config *CaptureConfig, baseSeqNo uint64, hostname string) ([]*UsageEvent, error) {
    logger.Infow("CaptureBothSources: Entry", "baseSeqNo", baseSeqNo, "hostname", hostname)

    ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
    defer cancel()

    events := make([]*UsageEvent, 0, 2)

    // Capture Claude
    logger.Info("CaptureBothSources: Capturing Claude")
    claudeEvent, err := runClaudeStatus(ctx, config)
    if err != nil {
        logger.Errorw("CaptureBothSources: Claude capture failed", "err", err)
        // Continue to try Codex even if Claude fails
    } else {
        enriched := enrichEvent(claudeEvent, baseSeqNo, hostname)
        events = append(events, enriched)
    }

    // Capture Codex
    logger.Info("CaptureBothSources: Capturing Codex")
    codexEvent, err := runCodexStatus(ctx, config)
    if err != nil {
        logger.Errorw("CaptureBothSources: Codex capture failed", "err", err)
    } else {
        enriched := enrichEvent(codexEvent, baseSeqNo+1, hostname)
        events = append(events, enriched)
    }

    if len(events) == 0 {
        return nil, fmt.Errorf("both Claude and Codex captures failed")
    }

    logger.Infow("CaptureBothSources: Success", "count", len(events))
    return events, nil
}
```

Option B: Parallel capture (faster, more complex):
```go
// CaptureBothSourcesParallel runs Claude + Codex captures concurrently
func CaptureBothSourcesParallel(config *CaptureConfig, baseSeqNo uint64, hostname string) ([]*UsageEvent, error) {
    ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
    defer cancel()

    type result struct {
        event *UsageEvent
        err   error
        source string
    }

    results := make(chan result, 2)

    // Launch Claude capture
    go func() {
        event, err := runClaudeStatus(ctx, config)
        results <- result{event: event, err: err, source: "claude"}
    }()

    // Launch Codex capture
    go func() {
        event, err := runCodexStatus(ctx, config)
        results <- result{event: event, err: err, source: "codex"}
    }()

    // Collect results
    events := make([]*UsageEvent, 0, 2)
    seqNo := baseSeqNo

    for i := 0; i < 2; i++ {
        r := <-results
        if r.err != nil {
            logger.Errorw("Capture failed", "source", r.source, "err", r.err)
            continue
        }
        enriched := enrichEvent(r.event, seqNo, hostname)
        events = append(events, enriched)
        seqNo++
    }

    if len(events) == 0 {
        return nil, fmt.Errorf("both captures failed")
    }

    return events, nil
}
```

**Recommendation**: Start with **Option A (Sequential)** for safety, optimize to Option B later if needed.

### Phase 3: Update Sampling Loop (20 minutes)

**Task 3.1: Modify runSamplingLoop to handle multiple events**

File: `cmd/sampler/main.go`

Current code around line 233:
```go
// OLD CODE (single event):
event, err := CaptureUsage(config.Capture, seqNo, config.Hostname)
if err != nil {
    // handle error
}
if err := appender.Append(event); err != nil {
    // handle error
}
```

New code:
```go
// NEW CODE (multiple events):
events, err := CaptureBothSources(config.Capture, seqNo, config.Hostname)
if err != nil {
    logger.Errorw("Capture failed", "err", err)
    metrics.RecordCaptureFailed()

    // Increment consecutive failures
    if err := stateManager.IncrementFailures(); err != nil {
        log.Printf("ERROR: Failed to update failure count: %v", err)
    }

    // Calculate backoff
    state := stateManager.GetState()
    backoff := calculateBackoff(state.ConsecutiveFailures)
    nextRun = time.Now().Add(config.Cadence).Add(backoff)

    log.Printf("Capture failed (%d consecutive), next run with backoff: %v",
        state.ConsecutiveFailures, nextRun)
    continue
}

// Append all captured events to JSONL
for _, event := range events {
    if err := appender.Append(event); err != nil {
        log.Printf("ERROR: Failed to append event (source=%s): %v",
            event.SourceTool, err)
        // Continue with other events even if one fails
    }
}

// Reset consecutive failures on success
if err := stateManager.ResetFailures(time.Now()); err != nil {
    log.Printf("ERROR: Failed to reset failures: %v", err)
}

// Update metrics
metrics.RecordCaptureSuccess()
for _, event := range events {
    metrics.UpdateLastCapture(event.SourceTool, event.Timestamp)
}
```

**Task 3.2: Update StateManager to track both sources**

File: `cmd/sampler/state.go`

Current state only tracks global seq_no. Consider adding per-source tracking (optional):
```go
type SamplerState struct {
    SeqNo               uint64    `json:"seq_no"`
    ConsecutiveFailures int       `json:"consecutive_failures"`
    LastCaptureTime     time.Time `json:"last_capture_time"`

    // NEW: Per-source tracking (optional, for debugging)
    LastClaudeSeqNo     uint64    `json:"last_claude_seq_no,omitempty"`
    LastCodexSeqNo      uint64    `json:"last_codex_seq_no,omitempty"`
}
```

### Phase 4: Metrics Updates (15 minutes)

**Task 4.1: Add per-source capture metrics**

File: `cmd/sampler/metrics.go`

Add new metrics:
```go
type Metrics struct {
    // Existing metrics...

    // NEW: Per-source capture tracking
    CapturesTotal      *prometheus.CounterVec  // {source="claude|codex"}
    CaptureFailures    *prometheus.CounterVec  // {source="claude|codex"}
    LastCaptureTime    *prometheus.GaugeVec    // {source="claude|codex"}
}

func NewMetrics() *Metrics {
    m := &Metrics{
        // ... existing metrics ...

        CapturesTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "cctrack_sampler_captures_total",
                Help: "Total number of successful captures by source",
            },
            []string{"source"},
        ),

        CaptureFailures: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "cctrack_sampler_capture_failures_total",
                Help: "Total number of failed captures by source",
            },
            []string{"source"},
        ),

        LastCaptureTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "cctrack_sampler_last_capture_timestamp_seconds",
                Help: "Timestamp of last successful capture by source",
            },
            []string{"source"},
        ),
    }

    prometheus.MustRegister(m.CapturesTotal)
    prometheus.MustRegister(m.CaptureFailures)
    prometheus.MustRegister(m.LastCaptureTime)

    return m
}

func (m *Metrics) UpdateLastCapture(source string, timestamp time.Time) {
    m.CapturesTotal.WithLabelValues(source).Inc()
    m.LastCaptureTime.WithLabelValues(source).Set(float64(timestamp.Unix()))
}

func (m *Metrics) RecordCaptureFailed(source string) {
    m.CaptureFailures.WithLabelValues(source).Inc()
}
```

### Phase 5: Testing & Validation (30 minutes)

**Task 5.1: Unit tests**

File: `cmd/sampler/capture_test.go`

```go
func TestCaptureBothSources(t *testing.T) {
    config := &CaptureConfig{
        ClaudeStatusPath: "/path/to/mock-claude-status",
        CodexStatusPath:  "/path/to/mock-codex-status",
        Timeout:          10 * time.Second,
    }

    events, err := CaptureBothSources(config, 100, "testhost")

    assert.NoError(t, err)
    assert.Len(t, events, 2, "should capture both sources")

    // Verify source tags
    sources := make(map[string]bool)
    for _, event := range events {
        sources[event.SourceTool] = true
    }

    assert.True(t, sources["claude"], "should have Claude event")
    assert.True(t, sources["codex"], "should have Codex event")

    // Verify seq_no monotonicity
    assert.Equal(t, uint64(100), events[0].SeqNo)
    assert.Equal(t, uint64(101), events[1].SeqNo)
}
```

**Task 5.2: Integration test**

```bash
# Start fresh sampler with dual capture
rm -f /var/lib/cctrack/usage-*.jsonl
rm -f /var/lib/cctrack/sampler-state.json

./bin/cctrack-sampler --data-dir /var/lib/cctrack \
    --claude-status ./bin/cctrack-claude-status \
    --codex-status ./bin/cctrack-codex-status \
    --cadence 1m

# Wait 2 minutes for 2 capture cycles
sleep 120

# Verify JSONL contains both sources
jq '.source_tool' /var/lib/cctrack/usage-*.jsonl | sort | uniq -c
# Expected output:
#   2 "claude"
#   2 "codex"

# Verify metrics endpoint
curl -s http://localhost:9089/metrics | grep cctrack_sampler_captures_total
# Expected:
#   cctrack_sampler_captures_total{source="claude"} 2
#   cctrack_sampler_captures_total{source="codex"} 2
```

**Task 5.3: End-to-end test (sampler → analyzer → widget)**

```bash
# Start analyzer
./bin/cctrack-analyzer --data-dir /var/lib/cctrack

# Wait for processing
sleep 10

# Check analyzer metrics
curl -s http://localhost:9090/metrics | grep codex_usage_ratio
# Expected: Non-empty values for 5h_limit and weekly_limit

curl -s http://localhost:9090/metrics | grep claude_usage_ratio
# Expected: Non-empty values for session, weekly_all, weekly_opus

# Check widget (visual verification)
# Widget should show:
#   - C5 dial with Codex 5h usage
#   - CW dial with Codex weekly usage
#   - S, W, O dials with Claude usage
```

### Phase 6: Error Handling & Edge Cases (20 minutes)

**Task 6.1: Partial failure handling**

Scenario: Claude capture succeeds, Codex fails
- Expected: Append Claude event, increment Codex failure counter
- Result: Claude metrics update, Codex shows stale/N/A

**Task 6.2: Binary missing**

Scenario: `cctrack-codex-status` not found
- Detection: `runCodexStatus()` returns exec error
- Mitigation: Log warning, continue with Claude-only mode
- Metric: Increment `codex_binary_missing` counter

**Task 6.3: Timeout handling**

Scenario: Codex capture takes >20 seconds
- Current: Context timeout cancels command
- Verify: Error logged, next cycle proceeds normally

**Task 6.4: Backward compatibility**

Scenario: Existing JSONL files with only Claude events
- Analyzer behavior: Correctly filters by per-source checkpoint
- Claude checkpoint: Advances normally
- Codex checkpoint: Remains at 0 until first Codex event arrives

### Phase 7: Documentation & Deployment (15 minutes)

**Task 7.1: Update README**

Add to configuration section:
```markdown
### Dual-Source Capture (Claude + Codex)

The sampler captures usage from both Claude CLI and Codex CLI every 5 minutes.

**Required binaries:**
- `cctrack-claude-status` (captures Claude `/status`)
- `cctrack-codex-status` (captures Codex `/status`)

**Configuration:**
```bash
CLAUDE_STATUS_PATH=/path/to/cctrack-claude-status \
CODEX_STATUS_PATH=/path/to/cctrack-codex-status \
./bin/cctrack-sampler
```

**Metrics exposed:**
- `cctrack_sampler_captures_total{source="claude"}` - Claude capture count
- `cctrack_sampler_captures_total{source="codex"}` - Codex capture count
```

**Task 7.2: Update ARCHITECTURE_PLAN_v2.md**

Section to update: "Decision 4: Data Collection Strategy"

Add subsection:
```markdown
#### Multi-Source Capture (v2.2)

Sampler captures both Claude and Codex usage in a single cycle:
- Sequential execution (Claude → Codex) for reliability
- Independent failure tracking per source
- Shared JSONL files with `source_tool` tagging
- Per-source checkpointing in analyzer (already implemented)
```

**Task 7.3: Create migration guide**

File: `docs/MIGRATION_DUAL_SOURCE.md`

```markdown
# Migration Guide: Single-Source → Dual-Source Capture

## What's Changing

Sampler now captures **both Claude and Codex** usage data.

## Impact

- JSONL files will contain events with `source_tool="codex"` (new)
- Sequence numbers increment by 2 per cycle (one for each source)
- Analyzer checkpoint now per-source (transparent to users)

## Migration Steps

1. **Build codex-status binary:**
   ```bash
   go build -o bin/cctrack-codex-status ./cmd/codex-status
   ```

2. **No data migration needed** - existing JSONL files compatible

3. **Restart sampler** with new config:
   ```bash
   systemctl restart cctrack-sampler
   ```

4. **Verify both sources capturing:**
   ```bash
   tail -f /var/lib/cctrack/usage-$(date +%Y%m%d).jsonl | jq '.source_tool'
   ```

## Rollback

To revert to Claude-only mode:
```bash
# Set CODEX_STATUS_PATH to empty or invalid path
CODEX_STATUS_PATH="" ./bin/cctrack-sampler
```

Sampler will log warnings but continue with Claude captures only.
```

**Task 7.4: Update systemd service**

File: `/etc/systemd/system/cctrack-sampler.service`

Add environment variables:
```ini
[Service]
Environment="CODEX_STATUS_PATH=/usr/local/bin/cctrack-codex-status"
Environment="CLAUDE_STATUS_PATH=/usr/local/bin/cctrack-claude-status"
```

## Rollout Plan

### Stage 1: Development (Day 1)
- [ ] Implement Phase 1-3 (infrastructure + capture logic)
- [ ] Local testing with mock binaries
- [ ] Code review

### Stage 2: Testing (Day 2)
- [ ] Unit tests (Phase 5.1)
- [ ] Integration tests (Phase 5.2)
- [ ] Error scenario testing (Phase 6)

### Stage 3: Staging Deployment (Day 3)
- [ ] Deploy to staging environment
- [ ] Run for 24 hours
- [ ] Monitor metrics for anomalies
- [ ] Verify widget displays Codex data

### Stage 4: Production Deployment (Day 4)
- [ ] Build production binaries
- [ ] Update systemd service
- [ ] Rolling restart sampler instances
- [ ] Monitor for 48 hours

## Success Criteria

1. **Functional:**
   - ✅ Widget shows 5 dials with live data (2 Codex + 3 Claude)
   - ✅ Both sources update every 5 minutes
   - ✅ Analyzer processes both sources independently
   - ✅ Per-source checkpointing works correctly

2. **Reliability:**
   - ✅ Partial failures don't block other source
   - ✅ Existing Claude-only JSONL files still processed
   - ✅ No data loss during rollout

3. **Performance:**
   - ✅ Dual capture completes within 30 seconds (avg)
   - ✅ No increase in memory usage (both sources buffered)
   - ✅ Prometheus metrics remain responsive

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Codex binary missing | Codex metrics stay N/A | Graceful degradation to Claude-only |
| Dual capture timeout | No data for cycle | Independent timeouts per source |
| Seq_no collision | Checkpoint corruption | Increment seq_no for each event |
| Widget doesn't show Codex | User confusion | Widget already supports Codex (no changes) |
| Analyzer rejects events | Data loss | Per-source checkpoint filters correctly |

## Open Questions

1. **Should captures run parallel or sequential?**
   - **Decision**: Sequential (Phase 2, Option A) for v1, optimize later
   - **Rationale**: Simpler error handling, less tmux socket contention

2. **What if only Codex fails repeatedly?**
   - **Decision**: Track per-source failure metrics, but don't block Claude
   - **Metric**: `cctrack_sampler_capture_failures_total{source="codex"}`

3. **Should seq_no be global or per-source?**
   - **Decision**: Global monotonic (current behavior preserved)
   - **Rationale**: Simpler checkpoint logic, matches existing design

4. **How to handle Codex CLI version changes?**
   - **Decision**: Same anchor validation as Claude (codex-status already has this)
   - **Golden fixtures**: Add Codex output samples to `test/parser-fixtures/`

## Future Enhancements

1. **Parallel capture** (Phase 2, Option B): 2x faster, needs goroutine management
2. **Dynamic source discovery**: Auto-detect available CLIs at runtime
3. **Per-source cadence**: Allow different sampling rates (Claude 5m, Codex 10m)
4. **Health dashboard**: Web UI showing per-source capture success rates

---

**Estimated Total Effort**: 2-3 hours implementation + 1 hour testing = **~4 hours**

**Priority**: High (user-reported bug: Codex shows N/A after wake)

**Dependencies**:
- None (codex-status already exists, analyzer ready, widget ready)

**Blocker Resolution**:
- Sampler only knows how to call one binary → Add second binary path + capture function
