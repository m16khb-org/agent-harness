# Verified Execution Evidence Contract

Every criterion must produce observable evidence from one of four Manual-QA channels. Tests alone NEVER prove done.

## The Contract

```
I, <agent name>, assert that criterion <id> is PASS.

Evidence type: <HTTP call | tmux | browser use | computer use>
Channel command: <exact command run>
Artifact path: <.issueops/verified-execution/evidence/<goal>-<criterion>.ext>
Artifact summary: <what the artifact proves>

Cleanup receipt:
  - <resource> → <action taken> → verified by <check>
  - ...

Metrics:
  - Rework count for this criterion: <N>
  - Attempts: <N>
  - Cycle: <current cycle of 5>

Signed: <timestamp>
```

## Channel-Specific Evidence Requirements

### HTTP call
```bash
curl -i <url> 2>&1 | tee .issueops/verified-execution/evidence/<goal>-<criterion>.txt
# Artifact MUST contain: HTTP status line, response headers, response body
```

### tmux
```bash
tmux new-session -d -s verified-execution-qa-<criterion>
tmux send-keys -t verified-execution-qa-<criterion> '<command>' Enter
sleep <N>  # wait for command to produce output
tmux capture-pane -t verified-execution-qa-<criterion> -pS -E - > .issueops/verified-execution/evidence/<goal>-<criterion>.txt
tmux kill-session -t verified-execution-qa-<criterion>
# Cleanup receipt: "tmux kill-session verified-execution-qa-<criterion>; verified tmux ls shows no session"
```

### Browser use
```
1. Open Chrome/agent-browser to <url>
2. Perform actions: <exact steps with selectors>
3. Take screenshot: .issueops/verified-execution/evidence/<goal>-<criterion>.png
4. Record action log: .issueops/verified-execution/evidence/<goal>-<criterion>-actions.txt
5. Close browser context
# Cleanup receipt: "browser context closed; no lingering chrome processes"
```

### Computer use
```
1. Launch <application>
2. Perform actions: <exact steps>
3. Take screenshot: .issueops/verified-execution/evidence/<goal>-<criterion>.png
4. Record action log: .issueops/verified-execution/evidence/<goal>-<criterion>-actions.txt
5. Close application
# Cleanup receipt: "application closed; PID <N> confirmed dead"
```

## Invalid Evidence (Reject Immediately)

- "Tests pass" without channel artifact
- `--dry-run` output
- "Should respond with..." (speculation, not observation)
- "Looks correct" (subjective, not binary)
- Worker self-report without re-verification
- Artifact path exists but file is empty or truncated
- Cleanup receipt missing
- Evidence from wrong channel (e.g., CLI dump for browser-facing criterion)

## Evidence Quality Standards

1. **Binary pass/fail**: The artifact must make PASS or FAIL obvious without interpretation.
2. **Reproducible**: Another agent reading the channel command must be able to re-run it.
3. **Complete**: No truncation. If output exceeds 32KB, note the truncation point.
4. **Named**: File name encodes goal + criterion + channel. Example: `G1-C2-http.txt`.
