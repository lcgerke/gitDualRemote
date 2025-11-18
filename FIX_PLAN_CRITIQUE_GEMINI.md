# Bug Analysis and Fix Plan: Enable Codex Data Capture

This document provides a detailed analysis of the bug preventing Codex metric collection and a plan to fix it.

## Bug Mechanics

The bug's root cause is in the `cmd/sampler/capture.go` file. The `CaptureUsage` function, which is responsible for collecting usage data, only contains a call to `runClaudeStatus`. There is no corresponding call to a function that would capture Codex data (e.g., `runCodexStatus`). This means that every time the sampler runs, it exclusively queries the Claude tool for its status, completely ignoring the Codex tool. The `cctrack-codex-status` binary exists and is functional, but it is never executed by the sampler.

## Data Flow Analysis

### Current (Broken) Data Flow
1.  The `sampler` process executes its main loop every 5 minutes.
2.  Inside the loop, it calls `CaptureUsage`.
3.  `CaptureUsage` executes only the `cctrack-claude-status` binary.
4.  The output of `cctrack-claude-status` is parsed into a `UsageEvent` with `source_tool="claude"`.
5.  This event is written to the `usage-YYYYMMDD.jsonl` file.
6.  The `analyzer` process reads the JSONL file, finds only "claude" events, and updates the `claude_*` Prometheus metrics.
7.  The widget/dashboard queries the metrics API and displays data for Claude but "N/A" for all Codex-related dials because the `codex_*` metrics are never populated.

### Target (Fixed) Data Flow
1.  The `sampler` process executes its main loop every 5 minutes.
2.  Inside the loop, it will call a new function, `CaptureBothSources`.
3.  `CaptureBothSources` executes both `cctrack-claude-status` and `cctrack-codex-status` sequentially.
4.  Two `UsageEvent` objects are created, one for each source (`source_tool="claude"` and `source_tool="codex"`), with sequential sequence numbers.
5.  Both events are written to the `usage-YYYYMMDD.jsonl` file.
6.  The `analyzer` reads the JSONL file, processes both events, and correctly updates both `claude_*` and `codex_*` Prometheus metrics.
7.  The widget/dashboard queries the metrics API and displays live data for both Claude and Codex.

## State Management

The primary state corruption is a sin of omission rather than incorrect data. The `usage-YYYYMMDD.jsonl` files, which represent the ground truth of usage, are in an incomplete state because they are missing all `codex` usage events. The `analyzer`'s internal state (checkpoints) is technically correct, as it maintains separate checkpoints for each `source_tool`, but the `codex` checkpoint never advances beyond its initial value of 0 because no Codex events are ever found.

## Test Cases

### Missing Tests
The current test suite is missing tests that verify the sampler's ability to handle multiple data sources.

### Proposed Test Cases
1.  **Unit Test for Dual Capture**: In `cmd/sampler/capture_test.go`, a new test `TestCaptureBothSources` should be added. This test will mock the `cctrack-claude-status` and `cctrack-codex-status` binaries and assert that the `CaptureBothSources` function returns two `UsageEvent` objects, one for each source, with correct and monotonic sequence numbers.

2.  **Integration Test for Sampler Output**: An integration test should be created to run the modified sampler for a few cycles and then inspect the output `usage-YYYYMMDD.jsonl` file. This test would use `jq` or a similar tool to verify that the file contains an equal number of events with `source_tool="claude"` and `source_tool="codex"`.

3.  **End-to-End Test**: A full end-to-end test should be performed by running the modified sampler, the analyzer, and then querying the analyzer's Prometheus metrics endpoint. This test must verify that the `codex_usage_ratio`, `codex_pace_ratio`, and other `codex_*` metrics are present and have non-zero values.

## Implementation Fix

The following code changes are required to fix the bug.

### 1. Update `cmd/sampler/config.go`
Add a `CodexStatusPath` field to the `CaptureConfig` struct and set its default value.

```go
// In type CaptureConfig struct:
type CaptureConfig struct {
    ClaudeStatusPath string        // Existing
    CodexStatusPath  string        // NEW: Path to cctrack-codex-status
    // ... rest of fields
}

// In func DefaultSamplerConfig():
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

### 2. Update `cmd/sampler/main.go`
Add a CLI flag for the new config option and modify the main sampling loop to call `CaptureBothSources` and handle multiple events.

```go
// In parseFlags():
flag.StringVar(&config.Capture.CodexStatusPath, "codex-status",
    getEnv("CODEX_STATUS_PATH", config.Capture.CodexStatusPath),
    "Path to cctrack-codex-status binary")

// In runSamplingLoop(), replace the existing CaptureUsage call:
// OLD: event, err := CaptureUsage(...)
// NEW:
events, err := CaptureBothSources(config.Capture, seqNo, config.Hostname)
if err != nil {
    // ... (error handling) ...
    continue
}

// Append all captured events
for _, event := range events {
    if err := appender.Append(event); err != nil {
        log.Printf("ERROR: Failed to append event (source=%s): %v",
            event.SourceTool, err)
    }
}
// ... (update metrics for each event) ...
```

### 3. Update `cmd/sampler/capture.go`
This is the core of the change. Add a new `runCodexStatus` function and a `CaptureBothSources` function that calls both capture functions.

```go
// New function to run codex-status
func runCodexStatus(ctx context.Context, config *CaptureConfig) (*UsageEvent, error) {
    // ... (implementation is similar to runClaudeStatus, but calls config.CodexStatusPath)
    // ... (it should parse the JSON output into a UsageEvent)
    var event UsageEvent
    if err := json.Unmarshal(output, &event); err != nil {
        return nil, fmt.Errorf("failed to parse codex-status JSON: %w", err)
    }
    return &event, nil
}

// New function to capture from both sources
func CaptureBothSources(config *CaptureConfig, baseSeqNo uint64, hostname string) ([]*UsageEvent, error) {
    ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
    defer cancel()

    events := make([]*UsageEvent, 0, 2)

    // Capture Claude
    if claudeEvent, err := runClaudeStatus(ctx, config); err != nil {
        logger.Errorw("Claude capture failed", "err", err)
    } else {
        enriched := enrichEvent(claudeEvent, baseSeqNo, hostname)
        events = append(events, enriched)
    }

    // Capture Codex
    if codexEvent, err := runCodexStatus(ctx, config); err != nil {
        logger.Errorw("Codex capture failed", "err", err)
    } else {
        // Increment sequence number for the second event
        enriched := enrichEvent(codexEvent, baseSeqNo+uint64(len(events)), hostname)
        events = append(events, enriched)
    }

    if len(events) == 0 {
        return nil, fmt.Errorf("both Claude and Codex captures failed")
    }

    return events, nil
}

// Helper function to enrich event with metadata
func enrichEvent(event *UsageEvent, seqNo uint64, hostname string) *UsageEvent {
    event.SeqNo = seqNo
    event.Hostname = hostname
    // event.Timestamp is assumed to be set by the status tool
    return event
}
```

## Verification

To confirm the fix works completely:
1.  **Build Binaries**: Build the `cctrack-sampler` and `cctrack-codex-status` binaries.
2.  **Run Sampler**: Start the `cctrack-sampler` with the paths to both status binaries configured. Let it run for at least one cycle (default is 5 minutes, can be lowered with `--cadence` for testing).
3.  **Inspect JSONL File**: Check the contents of the latest `usage-YYYYMMDD.jsonl` file in the data directory. It should contain two events per cycle, one with `"source_tool":"claude"` and another with `"source_tool":"codex"`.
4.  **Run Analyzer**: Start the `cctrack-analyzer`.
5.  **Check Metrics**: After the analyzer has had time to process the new events, access its Prometheus metrics endpoint (e.g., `curl http://localhost:9090/metrics`).
6.  **Confirm Codex Metrics**: Search the metrics output for `codex_` prefixed metrics (e.g., `cctrack_codex_usage_ratio`). These metrics must be present and have values corresponding to the data captured from the `cctrack-codex-status` tool. The presence of these metrics confirms the entire data pipeline is working as intended.
