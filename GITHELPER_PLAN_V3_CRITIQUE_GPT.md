# GitHelper Plan V3 Critique

## Assumptions

### Well-grounded
- Relying on Vault as the single configuration source and layering a cached copy with a visible staleness indicator gives the offline mode a solid foundation, because each change propagates through Vault, and the cached JSON is intentionally TTL-gated (24h) while secrets are intentionally fetched live (`GITHELPER_PLAN_V3.md`:320-380).
- Pivoting to the Git CLI wrapper after proving go-git could not configure `pushurl` was a defensible assumption that the native git binary is available and will behave consistently; the plan documents the requirement explicitly (`GITHELPER_PLAN_V3.md`:150-223).

### Risky
- The plan assumes Vault will be reachable often enough to seed caches and to distribute SSH keys; outside of the 24h TTL for configs there is no fall-back path for entirely offline first-time setups or for long-lived outages, yet the tool still expects to download keys before the first push (`GITHELPER_PLAN_V3.md`:320-380, 73-101).
- Automatic hook installation with silent backups assumes users never need to compose hooks or that simple restore instructions are sufficient; it does not account for existing hooks that rely on ordering or share variables, so overwriting them—even with backups—could break workflows unless the user inspects every backup before running again (`GITHELPER_PLAN_V3.md`:607-660).
- The security model leans heavily on Vault for secrets but leaves responsibility for Vault hardening, key rotation, and `VAULT_TOKEN` protection entirely to the user (`GITHELPER_PLAN_V3.md`:701-732); that pushes operational risk onto adopters without baked-in guidance.

## Gaps
- Vault authentication and credential rotation workflows are never documented; the plan shows how to `vault kv put` secrets but never explains how a new machine obtains `VAULT_TOKEN` or how tokens/keys are rotated in production, which makes the true deployment story underspecified (`GITHELPER_PLAN_V3.md`:320-380, 701-732).
- Partial failures are described (push to bare succeeds while GitHub is down) but there is no post-mortem or automated retry policy beyond telling users to run `githelper github sync --retry-github`; there is no record of how often failures happen or whether retries should be queued, which leaves the divergence detection/repair loops vague (`GITHELPER_PLAN_V3.md`:447-479, 467-478).
- The plan shows a comprehensive doctor output but omits how diagnosis results feed back into remediation; for example, how does `githelper doctor --credentials` help fix a stale token or a missing SSH key? That missing closure reduces the command to visibility only (`GITHELPER_PLAN_V3.md`:532-605).
- Hooks interact with `githelper github test`, but the dependency on network connectivity for pushing now causes a pre-push failure if GitHub is intermittently unreachable, yet the plan does not describe a grace period, caching of the last-known-good status, or an opt-out for unreliable networks (`GITHELPER_PLAN_V3.md`:628-637).

## Technical Feasibility
- The stack (Vault client, go-github, os/exec for git) is realistic; each area already has battle-tested libraries, and the plan acknowledges needing git >= 2.0 for `pushurl`, which is widely available in Linux distros, so the core pieces can be built with existing tooling (`GITHELPER_PLAN_V3.md`:209-223, 681-699).
- State management with a hybrid ownership model (git config authoritative, state file for metadata, Vault for secrets) is feasible, but the plan assumes that auto-repair of the state file via scanning repositories is efficient and deterministic; in practice, scanning hundreds of repos each run may be expensive and needs throttling or caching beyond what is described (`GITHELPER_PLAN_V3.md`:36-57, 382-401).
- Building a CLI around git via `os/exec` is technically sound, yet the plan states the CLI will shell out for every git operation without mentioning command batching or multiplexing; long-running commands or repeated calls (e.g., doctor hitting all repos) might suffer from latency if every repo requires spawning git repeatedly (`GITHELPER_PLAN_V3.md`:175-207, 228-254).

## Implementation Risks
- The hook installation strategy replaces hooks entirely even though it backs them up; that can silently break existing hooks that expect to run before or after githelper's checks, and relying on users to manually restore the backup shifts the burden rather than offering chaining or detection of composability issues (`GITHELPER_PLAN_V3.md`:607-660).
- Auto-downloading SSH keys to disk undermines the stated desire to track credentials; keys are stored with 600 permissions, but there is no plan for lifecycle management (rotate/delete) or for cleaning up keys when a repo is removed (`GITHELPER_PLAN_V3.md`:71-101, 403-421, 333-353).
- The doctor command claims to be comprehensive, but building accurate diagnostics over Vault, SSH, git config, hooks, and sync status requires running a lot of asynchronous checks—each of which can fail for timing, permissions, or networking reasons; without retry windows or circuit breakers, a flaky network could make doctor report failures while the system is actually healthy (`GITHELPER_PLAN_V3.md`:532-569, 774-809).
- Dependency on Vault for configuration makes the tool brittle if Vault upgrades change the schema; there is no versioning strategy for the cached `config.json` or for schemas stored in Vault, so Vault changes could silently break githelper until support is manually added (`GITHELPER_PLAN_V3.md`:320-380, 368-380).

## Timeline Realism
- The phased breakdown (Phase 1–4 each one week, Phase 5 taking 3-4 weeks) underestimates the integration complexity because each phase touches multiple subsystems; for example, Phase 2 not only needs Vault/API wiring but also repository-local SSH config, dual-push, and hook installation, which realistically could take 2+ weeks once unforeseen edge cases appear (`GITHELPER_PLAN_V3.md`:745-761, 781-809).
- Phase 5's 3-4 weeks for comprehensive testing, mock servers, and documentation is optimistic because it simultaneously tasks the team with mocking external services, writing fixtures, and fully documenting workflows; even with two people (author + Claude), coordinating those mocks, especially for Vault and GitHub, needs more than four weeks (`GITHELPER_PLAN_V3.md`:781-809).
- The timeline omits buffer for operations such as Vault rollout, user feedback on hooks, or pivoting after discovery of new blockers (Phase 0 already caused a pivot to the CLI wrapper), so the 7-8 week total may slip if anything breaks besides the ones already captured (`GITHELPER_PLAN_V3.md`:896-922).

## Concrete Improvements
1. **Document Vault onboarding/rotation**: add a sub-section that describes how to obtain `VAULT_TOKEN`, rotate SSH keys/PATs, and how githelper detects mismatched secrets, reducing the implicit user responsibility baked into the security model (`GITHELPER_PLAN_V3.md`:320-380, 701-732).
2. **Formalize hook composability**: instead of overwriting existing hooks, consider chaining (run existing hook, capture result, then run githelper hook) or at least include a `githelper hooks status` command that lists what hooks were overwritten and whether they succeeded during the last push (`GITHELPER_PLAN_V3.md`:607-660).
3. **Extend the retry policy**: define how `githelper github sync --retry-github` behaves when GitHub is still down—should it retry in the background, schedule a cron, or only run once? This makes the divergence recovery path actionable instead of a manual instruction (`GITHELPER_PLAN_V3.md`:447-479, 763-771).
4. **Add instrumentation for cache-state drift**: track how often the state file auto-repairs from git config, log the delta, and surface this via `doctor` so you can tell whether manual reconciliation is needed instead of silently overwriting the cache (`GITHELPER_PLAN_V3.md`:36-57, 382-401).
5. **Buffer the timeline**: extend Phase 2/3 by another week and reserve a final review/feedback slot after Phase 5 to address integration learnings, increasing the total to 9–10 weeks but aligning expectations with the work described (`GITHELPER_PLAN_V3.md`:745-809, 896-922).
