#!/usr/bin/env bash
# Read a claude.ai/code artifact (or any iframe-rendered public page) through
# the installed Aside browser and write its text + HTML to OUT_DIR.
#
# Usage: read-artifact.sh <artifact-url> [out-dir] [timeout-seconds]
# Output: <out-dir>/artifact.txt, <out-dir>/artifact.html, and one
#         "ARTIFACT_READ_RESULT {json}" line on stdout.
set -euo pipefail

url="${1:?usage: read-artifact.sh <artifact-url> [out-dir] [timeout-seconds]}"
out_dir="${2:-./artifact-read}"
timeout_s="${3:-45}"

case "$url" in
  http://*|https://*) ;;
  *) echo "ARTIFACT_READ_RESULT {\"status\":\"error\",\"reason\":\"url must start with http(s)://\"}"; exit 2 ;;
esac

command -v aside >/dev/null || { echo 'ARTIFACT_READ_RESULT {"status":"error","reason":"aside CLI not installed"}'; exit 2; }
command -v python3 >/dev/null || { echo 'ARTIFACT_READ_RESULT {"status":"error","reason":"python3 required"}'; exit 2; }
mkdir -p "$out_dir"

# One REPL invocation = one JS context. Everything (open → poll → extract →
# close) lives in this single script so the test-owned tab can never leak.
url_json="$(URL="$url" python3 -c 'import json,os; print(json.dumps(os.environ["URL"]))')"
read -r -d '' js <<JS || true
const url = ${url_json};
const timeoutMs = ${timeout_s} * 1000;
const p = await openTab(url);
try {
  await p.waitForLoadState('domcontentloaded');
  const MIN_CHARS = 200;
  const pick = async () => {
    let best = null;
    for (const f of p.frames()) {
      let text = '';
      try { text = await f.evaluate(() => document.body ? document.body.innerText : ''); } catch (_) { continue; }
      if (!best || text.length > best.text.length) best = { frame: f, text };
    }
    return best;
  };
  const start = Date.now();
  let best = await pick();
  while ((!best || best.text.length < MIN_CHARS) && Date.now() - start < timeoutMs) {
    await new Promise(r => setTimeout(r, 500));
    best = await pick();
  }
  const title = await p.title();
  const topText = await p.evaluate(() => document.body.innerText);
  const ok = !!best && best.text.length >= MIN_CHARS;
  const notFound = /not found/i.test(title);
  const gated = !ok && !notFound && /Sign in|로그인/.test(topText);
  const html = best ? await best.frame.evaluate(() => document.documentElement.outerHTML) : '';
  const enc = s => (typeof Buffer !== 'undefined') ? Buffer.from(s, 'utf8').toString('base64') : btoa(Array.from(new TextEncoder().encode(s), b => String.fromCharCode(b)).join(''));
  console.log('ARTIFACT_READ_PAYLOAD ' + JSON.stringify({
    status: ok ? 'ok' : notFound ? 'not_found' : gated ? 'gated' : 'empty',
    title,
    url: await p.url(),
    frameUrl: best ? best.frame.url() : null,
    frameCount: p.frames().length,
    textLength: best ? best.text.length : 0,
    text: enc(best ? best.text : ''),
    html: enc(html),
  }));
} finally {
  await p.close();
}
JS

raw="$(aside repl "$js" 2>&1 || true)"
payload="$(printf '%s\n' "$raw" | grep -m1 '^ARTIFACT_READ_PAYLOAD ' | sed 's/^ARTIFACT_READ_PAYLOAD //' || true)"

if [ -z "$payload" ]; then
  printf '%s\n' "$raw" | tail -n 20 >&2
  echo 'ARTIFACT_READ_RESULT {"status":"error","reason":"no payload from aside repl (see stderr)"}'
  exit 1
fi

OUT_DIR="$out_dir" python3 - "$payload" <<'PY'
import base64, json, os, sys
d = json.loads(sys.argv[1])
out = os.environ["OUT_DIR"]
txt = os.path.join(out, "artifact.txt")
html = os.path.join(out, "artifact.html")
open(txt, "w", encoding="utf-8").write(base64.b64decode(d.pop("text")).decode("utf-8"))
open(html, "w", encoding="utf-8").write(base64.b64decode(d.pop("html")).decode("utf-8"))
d.update({"textPath": txt, "htmlPath": html})
print("ARTIFACT_READ_RESULT " + json.dumps(d, ensure_ascii=False))
sys.exit(0 if d["status"] == "ok" else 3)
PY
