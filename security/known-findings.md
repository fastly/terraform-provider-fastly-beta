# Known findings

Findings that were triaged as won't-fix or working as intended. Automated and LLM-driven scans should read this file first and not re-report listed items.

Each entry has one of two dispositions:

- **Won't fix** — the behavior is understood and accepted; no code change is planned.
- **Intended behavior** — the code works as designed. Re-check the entry if the referenced code changes materially.

## Entries

### Debug mode exposes sensitive API bodies

- **Disposition:** Intended behavior
- **Area:** `internal/client/data.go`, `internal/provider/provider.go`
- **Why:** `FASTLY_DEBUG_MODE=true` is an explicit operator opt-in whose purpose is dumping HTTP bodies, the same class as `curl -v` or `kubectl -v=9`. Bodies can include values the schema marks sensitive; the opt-in is the control.

### Lifecycle tests expose the Fastly API token

- **Disposition:** Won't fix
- **Area:** `scripts/test-lifecycle-cdn/run.sh`, `scripts/test-lifecycle-compute/run.sh`
- **Why:** Developer-only test scripts, not shipped provider code and not run by CI. Exposure is local to the host and bounded by the run.

### Empty cleanup prefix deletes every service

- **Disposition:** Won't fix
- **Area:** `scripts/cleanup-test-services/main.go`
- **Why:** No caller passes an empty prefix; the scheduled workflow uses the reserved `tf-test-` default. Choosing a broad prefix is the documented purpose of the override, and `TESTING.md` already documents the name-match hazard.

### PUT retry duplicates version-clone operations

- **Disposition:** Won't fix
- **Area:** `internal/client/data.go` (reached from `versionclone`, `servicecdnauto`, `servicecomputeauto`)
- **Why:** Occurs only on an ambiguous clone failure (the clone commits but the response is lost). The duplicate is an inactive draft version, so there is no production-traffic impact, and the provider cannot safely adopt an orphan without a unique operation identifier.

### Duplicate order entries inflate remote state

- **Disposition:** Won't fix
- **Area:** `internal/reconcile/reconcile.go` (`MatchOrder`)
- **Why:** Requires a duplicate nested-block name, which is already a configuration mistake. No security impact; Fastly object names are unique.

### Unknown target name is rejected as missing

- **Disposition:** Won't fix
- **Area:** `internal/validation/refs.go` (`NameSet`, `References`)
- **Why:** Appears only when a reference is known while the target it names is unknown, which means the operator already knows the name. The shape is uncommon and the workaround is trivial; no security impact.

### Health-check authorization headers are visible

- **Disposition:** Intended behavior
- **Area:** `internal/resources/healthcheck/schema.go` (`headers`)
- **Why:** `healthcheck.headers` is deliberately not marked sensitive: bearer-token health checks are niche, and masking the whole set would hide ordinary headers too. The attribute description warns that values appear in plan output and state.

### Removing the zone transfer block is ignored

- **Disposition:** Won't fix
- **Area:** `internal/resources/dnszone/expand.go` (`BuildUpdateInput`, `xfr_config_inbound`)
- **Why:** The DNS zone PATCH has no representation that clears the whole `xfr_config_inbound` object, so removing the block cannot be expressed. The apply fails with an inconsistent-result error rather than silently succeeding, and the workaround is to clear `inbound_tsig_key_id` inside a retained block or replace the zone. Forcing zone replacement on block removal was rejected: it deletes and recreates a live DNS zone and can change nameservers, which is worse than the stale trust it addresses. Revisit if the API gains a clear operation.

### Removing client IP headers is ignored

- **Disposition:** Won't fix
- **Area:** `internal/resources/ngwafworkspace/expand.go` (`BuildUpdateInput`, `client_ip_headers`)
- **Why:** Neither explicit null nor an empty array reliably clears the list on the NGWAF workspace API, so removal cannot be expressed. Forcing workspace replacement was rejected: it recreates every workspace-scoped NGWAF object and briefly drops protection. Revisit if the API gains a clear representation.
