# Fixed Plan Critique (GPT)

## Root Cause Analysis
- The written problem statement says Codex metrics stay at `N/A` because the sampler never runs `cctrack-codex-status`, so no Codex events reach the JSONL archive or Prometheus (FIX_PLAN.md:5).
- The analyzer already has Codex metric types and consumes `source_tool="codex"`, and the widget already requests Codex dials, so the missing data is confined to sampler capture logic (FIX_PLAN.md:9, FIX_PLAN.md:10, FIX_PLAN.md:11).
- Because the sampler only calls `cctrack-claude-status`, every recorded event carries `source_tool="claude"`, so the Codex JSONL entries and Prometheus gauges never populate (FIX_PLAN.md:30, FIX_PLAN.md:13, FIX_PLAN.md:14).

## Reproduction Steps
1. Confirm the sampler is configured to execute only the Claude binary, so it records Claude-only events every five minutes (current data flow begins at FIX_PLAN.md:26).
2. Trigger a wake-from-sleep cycle; the sampler still runs its five-minute cadence, but the Codex binary is never invoked, so no Codex event is emitted.
3. Tail the daily `usage-YYYYMMDD.jsonl` file and observe that each entry continues to declare `"source_tool":"claude"` (FIX_PLAN.md:581). This proves Codex is reproducibly missing because the sampler never runs the Codex capture path.

## Impact Assessment
- **What breaks:** Every Codex dial and gauge stays at `N/A` or stale values because the sampler never emits Codex events, even though the analyzer/widget already expect Codex data (FIX_PLAN.md:5, FIX_PLAN.md:9, FIX_PLAN.md:11).
- **Scope:** All Codex users are affected immediately after any wake/restart because no Codex data is written to `/var/lib/cctrack/usage-*.jsonl`, and the sampler is the only source of truth for these metrics.
- **Downstream:** Prometheus metrics, dashboards, and alerts keyed to Codex are dead because `codex_*` gauges never populate when only Claude data exists (FIX_PLAN.md:12, FIX_PLAN.md:14).

## Potential Fixes
- **Sequential Dual Capture (documented plan):** Extend the sampler to call both Claude and Codex binaries every cycle, writing one JSONL event per source before returning to sleep (capture addition described at FIX_PLAN.md:71 and dual capture logic starting at FIX_PLAN.md:105). Tradeoff: slight increase in cycle duration but keeps the existing checkpoint logic untouched.
- **Parallel Capture (future idea):** Run both binaries concurrently (mentioned under future enhancements at FIX_PLAN.md:683). This would reduce latency but requires goroutine coordination, timeout isolation, and shared-resource guards such as tmux sockets.
- **Conditional capture path:** Only invoke Codex when a health check or feature flag is set. It reduces runtime overhead but risks missing data if the condition is evaluated incorrectly or disabled inadvertently.

## Prevention
- **Automated smoke test:** Add an integration test that runs the sampler against mocked Claude/Codex binaries and verifies two records per cycle with distinct `source_tool` tags, similar to the parser fixtures noted for Codex (FIX_PLAN.md:679).
- **Observability:** Emit a per-source failure metric like `cctrack_sampler_capture_failures_total{source="codex"}` so regressions trigger alerts before dashboards go stale (a metric of this kind is already suggested at FIX_PLAN.md:671).
- **Config validation:** Fail-fast (with a logged warning) when `CodexStatusPath` is unset or invalid, mirroring the rollback behavior that gracefully degrades to Claude-only capture (FIX_PLAN.md:591). This prevents silent misconfiguration.

## Specific Recommendations
1. **Capture configuration (FIX_PLAN.md:71):** Add `CodexStatusPath` into `CaptureConfig`, default it alongside the Claude path, and expose the new CLI flag so operators can override it.
2. **Command execution (FIX_PLAN.md:105):** Implement `runCodexStatus()` analogous to `runClaudeStatus()`, respecting column/line limits and tmux socket flags, tagging the parsed JSON with `source_tool:"codex"` before writing.
3. **Sampler loop (FIX_PLAN.md:237):** Update the loop to invoke the Claude and Codex capture functions sequentially, bumping `seq_no` between writes and ensuring an error in one path does not abort the other (success criteria highlight the need for independent reliability at FIX_PLAN.md:637).
4. **Checkpoint writing (FIX_PLAN.md:305):** Continue tracking per-source checkpoints so the analyzer’s per-source logic remains valid when it reads a mix of Claude and Codex events.
5. **Systemd/service docs (FIX_PLAN.md:602):** Document setting both `CLAUDE_STATUS_PATH` and `CODEX_STATUS_PATH` via environment variables so deployments stay consistent.
6. **Verification (FIX_PLAN.md:581):** After deployment, tail the active usage file to ensure entries alternate between `claude` and `codex` `source_tool` values every five minutes.

## Debugging Guidance
- Run the sampler locally with both binary paths and inspect the JSONL file to confirm two entries per interval, reusing the tail+`jq '.source_tool'` verification pattern referenced at FIX_PLAN.md:586.
- If Codex events disappear in production, check sampler logs for warnings about missing binaries (graceful degradation is already described at FIX_PLAN.md:591) and verify the service environment variables are correct (FIX_PLAN.md:602).
- Once Codex capture is live, monitor `cctrack_sampler_capture_failures_total{source="codex"}` to detect timeouts or repeated errors (metric idea referenced at FIX_PLAN.md:671).

## Summary
The bug is entirely due to the sampler never running the Codex-specific binary, so Codex usage data is never recorded or exposed (FIX_PLAN.md:5). Implementing the dual-capture infrastructure and verification steps outlined in the plan (starting around FIX_PLAN.md:71) and adding the suggested tests/metrics will close the gap and prevent regression.
