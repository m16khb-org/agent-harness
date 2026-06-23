---
name: web-fetch-live-parity.md
description: Opt-in live parity procedure for the web-fetch benchmark and generic baseline comparator.
---

# Web-Fetch Live Parity

Live parity is an explicit opt-in check for the clean-room web-fetch engine. It is not part of the deterministic CI battery because it uses public network access and third-party availability can change.

## Scope

- Candidate command: `agent-harness web-fetch benchmark --fixtures testdata/webfetch/live/public-fixtures.json --live --json`
- Fixture file: `testdata/webfetch/live/public-fixtures.json`
- Optional baseline command: any executable that accepts `--url URL --json` and prints at least:

```json
{
  "ok": true,
  "category": "strong_ok"
}
```

The baseline protocol is intentionally source-neutral. Do not expose implementation-specific comparator names in public CLI, JSON, skills, or docs.

## Safety Boundary

Live parity must preserve the web-fetch safety model:

- Require `HARNESS_WEBFETCH_LIVE=1`.
- Do not bypass login, paywalls, CAPTCHA, WAF challenge pages, auth walls, or robots-sensitive restrictions.
- Treat `auth_required`, `paywalled`, `challenge`, and `blocked` as explicit limitations, not fetch failures to evade.
- Keep deterministic tests free of live network access.

## Commands

Candidate-only live run:

```bash
HARNESS_WEBFETCH_LIVE=1 ./bin/agent-harness web-fetch benchmark \
  --fixtures testdata/webfetch/live/public-fixtures.json \
  --live \
  --json
```

Candidate plus generic baseline run:

```bash
HARNESS_WEBFETCH_LIVE=1 ./bin/agent-harness web-fetch benchmark \
  --fixtures testdata/webfetch/live/public-fixtures.json \
  --live \
  --compare-baseline /path/to/baseline-fetch \
  --json
```

## Report Fields

Record these fields from the JSON output for each opt-in run:

- `live_parity_report.success_rate`
- `live_parity_report.category_agreement`
- `live_parity_report.route_count`
- `live_parity_report.latency_p50_ms`
- `live_parity_report.latency_p95_ms`
- `live_parity_report.safety_failures`
- `live_parity_report.false_strong_ok`
- `live_parity_report.baseline_available`
- `live_parity_report.baseline_success_rate`
- `live_parity_report.baseline_latency_p50_ms`
- `live_parity_report.baseline_latency_p95_ms`
- `live_parity_report.warnings`

## CI Candidate Gate

Do not promote live parity into CI until there are 7 consecutive opt-in runs with:

- `safety_failures == 0`
- `false_strong_ok == 0`
- `category_agreement >= 85` where comparable
- candidate success rate at least matches the baseline, or any lower success rate is justified by safer classification
- candidate `latency_p95_ms <= 125%` of baseline p95 unless the slower behavior prevents false success

If a live run fails because a public fixture changed, update the fixture only after confirming the new source behavior in a browser or with a plain `curl -I`/`curl` read that does not bypass access controls.
