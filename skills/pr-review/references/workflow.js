// PR Review inspection workflow (Claude Code `Workflow` tool). Pass via `scriptPath`, with
// args = contents of <out_dir>/workflow_args.json written by scripts/mr_context.py:
//   { outDir, checkout, codegraph, lenses, lensText, units, maxCandidates, perLensCap, hunkRanges,
//     refutedHistory, carried, models }
//   units[] = { id, lenses: [...], pack, files } — one finder per (lens bundle × shard); the pack is read once per bundle.
// The coordinator (you) builds the context pack BEFORE this runs and merges/posts AFTER.
//
// Design notes (measured on !5617, 2026-08-28; see .issueops/research/llm-code-review-tools-2026-08.md):
// - An agent's spend ≈ Σ over its turns of (context length) — so every unit reads ONE self-contained pack
//   and has a hard message budget. Packs cut finder cache-read 146.6M → 28.3M on the same MR, same model.
// - Cheap models did not use fewer tokens (sonnet finder 31 turns ≈ opus 33), so all roles run on the
//   session model unless args.models says otherwise. Cost levers are structural, not model choice.
// - The tracer sees the candidate WITHOUT the finder's evidence/upstream/downstream (information asymmetry,
//   OpenCodeReview arXiv 2608.09290) so it cannot merely re-read the finder's trace; the reproducer, which
//   runs only when the tracer failed to refute, gets the full candidate.
export const meta = {
  name: 'pr-review-inspection',
  description: 'Parallel multi-lens MR inspection with adversarial verification',
  phases: [
    { title: 'Find', detail: 'one inspector per unit (lens bundle × shard), each reading one self-contained pack; first unit warms the cache' },
    { title: 'Verify', detail: 'prescreen (code) → tracer (blind to finder evidence) → reproducer, early exit on refutation' },
  ],
}

const CANDIDATE = {
  type: 'object',
  properties: {
    path: { type: 'string' }, new_line: { type: 'integer' }, end_line: { type: ['integer', 'null'] },
    severity: { type: 'string', enum: ['critical', 'high', 'medium', 'low'] },
    category: { type: 'string', enum: ['bug', 'security', 'performance', 'business-logic', 'data', 'api-contract', 'test', 'rule', 'scope'] }, title: { type: 'string' }, what: { type: 'string' }, why: { type: 'string' }, how: { type: 'string' },
    evidence: { type: 'array', items: { type: 'string' } }, upstream: { type: 'string' }, downstream: { type: 'string' },
    suggestion: { type: ['string', 'null'] }, rule: { type: ['string', 'null'] }, confidence: { type: 'integer' },
    newly_reachable: { type: 'boolean' }, lens: { type: 'string' },
  },
  required: ['path', 'new_line', 'severity', 'category', 'title', 'what', 'why', 'how', 'evidence', 'confidence', 'lens'],
}
const FINDER = {
  type: 'object',
  properties: { lenses: { type: 'array', items: { type: 'string' } }, reviewed_files: { type: 'array', items: { type: 'string' } }, inspected: { type: 'array', items: { type: 'string' } },
    candidates: { type: 'array', items: CANDIDATE },
    verified_ok: { type: 'array', items: { type: 'object', properties: { concern: { type: 'string' }, why_ok: { type: 'string' }, loc: { type: 'string' }, thread: { type: ['string', 'null'] } }, required: ['concern', 'why_ok', 'loc'] } } },
  required: ['lenses', 'reviewed_files', 'inspected', 'candidates', 'verified_ok'],
}
const VERDICT = {
  type: 'object',
  properties: { skeptic: { type: 'string' }, refuted: { type: 'boolean' }, confidence: { type: 'integer' }, reason: { type: 'string' },
    evidence: { type: 'array', items: { type: 'string' } }, severity_adjust: { type: 'string', enum: ['keep', 'lower', 'raise'] },
    corrected_line: { type: ['integer', 'null'] }, corrected_suggestion: { type: ['string', 'null'] } },
  required: ['skeptic', 'refuted', 'confidence', 'reason', 'evidence', 'severity_adjust'],
}

const { outDir, checkout, codegraph, lensText, units = [], maxCandidates = 24, perLensCap = 3, hunkRanges = {}, refutedHistory = [], carried = [] } = args
// The level is a contract, not a label: post_review.py tells the author which verification ran,
// so a `high` pack must never reach the fan-out that would make that disclosure understate it.
// Levels below max are executed inline by the coordinator (SKILL.md § 2 Find — inline).
if (args.level && args.level !== 'max') throw new Error(`workflow.js runs the max pipeline; workflow_args.json says level=${args.level}. Re-run preflight with --level max, or run the inline path from SKILL.md instead.`)
const M = args.models || {}
// Omo native has its own pinned adapter path. Keep direct workflow invocations from inheriting
// the Claude/OpenCode `opus` defaults in workflow_args.json.
const isOmoNative = typeof process !== 'undefined' && process.env?.OMO_NATIVE === '1'
const role = (name) => isOmoNative ? 'zai/glm-5.3-flash' : args.model || M[name]
const FINDER_TURNS = 10
const SKEPTIC_TURNS = 8
const hunkSlug = (p) => p.replace(/\//g, '__').replace(/[^A-Za-z0-9_.\-]/g, '_')
const cg = codegraph ? '`codegraph explore "<symbol>"` prints definitions + call paths' : 'one `rg -n` call, then read the file region at the line it names'

// Prompt layout is deliberate: the long shared block comes first (identical for every unit, so the
// cached prefix is the same across agents), the per-unit tail (pack path, lenses) comes last.
const FINDER_COMMON = `You are one inspector in a formal design inspection (Parnas active design review).
Repository checkout at head: ${checkout} (read-only; never edit, never checkout, never post).

How to work — this is a budget, not advice:
- Your pack file (named at the end of this message) is the whole context for your slice: the cumulative diff of every file you inspect, the definition headers enclosing each hunk, files that historically change together, the definitions and one-hop callers/callees of the symbols they define, the rules that apply, existing threads and prior lessons.
- Message 1: exactly one Read of the pack file, whole (no offset/limit) — nothing else. Message 2: every follow-up read you need, all in that one message (several Read/Bash calls at once), chosen from the hops the pack names. Then at most a few more messages. Hard cap: ${FINDER_TURNS} assistant messages including the final StructuredOutput. One call per message wastes the budget. Read file regions (offset/limit or sed -n), never whole large files.
- You carry several lenses. Apply each one separately over the same pack and tag every candidate with the lens that found it (candidate.lens); a candidate two lenses would both report is reported once under the stronger lens. Lens-specific file scope: 'contract' looks at dto/controller/gateway/generated files, 'data' at db/repository/query code, 'async' at kafka/queue/stream/retry code, 'security' at auth/validation/secrets — skip files the lens does not apply to.
- Open the checkout only for a hop the pack names (a caller, a callee, a validator) or a symbol the pack does not list (${cg}). If ${outDir}/gate.md exists, lint/typecheck/test results and LOC metrics are already measured there — never re-report them.
- When the budget is nearly spent, stop and report what you verified; an unverified hunch is not a candidate.

Inspect ONLY through your lens. For every candidate defect you MUST, before reporting:
1. Open the real definition of every symbol the claim depends on — a diff hunk is not a definition.
2. Walk one hop upstream (who calls / what validates the input) and one hop downstream (who consumes the result) and state what you saw. If a hop is not findable (external library, dynamic dispatch), say so in upstream/downstream and cap confidence at 50 — do not guess it and do not silently drop the candidate.
3. Confirm the defect is on lines this change added/modified (hunks listed in the pack). If it is on an untouched line that the change makes newly reachable, set newly_reachable=true and say in why which changed line makes it reachable; otherwise the prescreen drops it.
Report a candidate only with a concrete failure scenario (input/state → wrong result). Return at most ${perLensCap} candidates PER LENS, strongest first — the cap is deliberate: the ${perLensCap} you can defend, not the first ${perLensCap} you noticed. Confidence follows the rubric (25 inferred / 50 verified-but-rare / 75 verified on a real path / 90+ proven with no escape upstream or downstream), not enthusiasm.
Do NOT report: style, naming, things a linter/typechecker/CI catches, pre-existing issues on untouched lines, speculative refactors, "consider adding" without a failure scenario, anything on a line that already has a thread in the pack unless you contradict that thread, anything listed under prior lessons.
verified_ok: things that looked risky and you cleared — {concern: "<우려, 한 구절>", why_ok: "<왜 괜찮은지, 한 문장, 근거 파일:라인 포함>", loc: "file:line", thread: "<existing thread id you are contradicting, else null>"}. Max 3; prefer ones that contradict an existing thread.
suggestion is REQUIRED when category is api-contract or the fix is a decorator/description/config one-liner.
All prose fields in Korean; identifiers/paths verbatim. evidence entries: "path:line — what it proves". Return lenses = the lens ids you applied.
reviewed_files is a coverage receipt: copy every file path listed in your unit's pack exactly once,
even when it produced no candidate. Do not put symbols or extra paths in reviewed_files; inspected
may still list files and symbols actually investigated in depth.`

const finderPrompt = (u) => `${FINDER_COMMON}

Your unit: ${u.id}
Pack file (message 1: one Read, whole — it lists the files in your slice): ${outDir}/${u.pack}
Lenses:
${u.lenses.map((l) => `- ${l} — ${lensText[l]}`).join('\n')}`

const SKEPTIC_TEXT = {
  tracer: 'Prove the claim rests on an inferred shape, an unchecked boundary, or a misreading of intent. (1) Open the real definition of every symbol in the claim. (2) Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto/schema constraints, caller preconditions) and downstream to the consumer of the result; report each hop as path:line. If any hop neutralises the scenario, refute. (3) Check the MR description (pack header) and the linked issue: is the behavior intentional AND correct with respect to what the issue asks? Intentional but wrong per the issue is still a defect — do not refute on intent alone.',
  reproducer: 'Prove the failure scenario cannot actually happen. Try to make it happen: write a throwaway unit test in the checkout encoding the scenario and run it, or run the targeted typecheck/lint on the file, or execute a small script. Paste command and outcome. Delete throwaway files afterwards. A scenario that cannot be reproduced and cannot be argued from definitions is refuted.',
}

// The tracer is blind to the finder's trace: it gets the claim and the scenario, not the evidence, so its
// hops are its own. The reproducer (second stage) sees everything, including the tracer's verdict.
const blind = (c) => ({ path: c.path, new_line: c.new_line, end_line: c.end_line, severity: c.severity, category: c.category, title: c.title, what: c.what, why: c.why, newly_reachable: c.newly_reachable, lens: c.lens })

const skepticPrompt = (id, c, prior) => `You are a skeptic in a design inspection. Your job is to try to REFUTE this candidate defect.
refuted=true ONLY when you actually neutralised the scenario with evidence (a definition, a boundary, a run that shows it cannot happen). If you could not test it (no runnable environment, missing DB, tooling absent) or could not find the hop, return refuted=false with confidence ≤ 40 and reason starting with "미확인:" — inability to verify is not a refutation.
Budget: at most ${SKEPTIC_TURNS} assistant messages including the final StructuredOutput; batch independent reads in one message.
Checkout (read-only except throwaway test files you delete afterwards): ${checkout}
Start here, in one message: ${outDir}/hunks/${hunkSlug(c.path)}.patch (the diff of the candidate's file) and the "## <symbol>" sections of ${outDir}/defs.md for the symbols in the claim (grep -n "^## " to locate). Open other files only for a hop those name.
Lens: ${id} — ${SKEPTIC_TEXT[id]}
Candidate: ${JSON.stringify(id === 'tracer' ? blind(c) : c)}${prior ? `\nThe tracer already examined it and did not refute (confidence ${prior.confidence}): ${prior.reason}. Do not repeat its trace; attack the scenario itself.` : ''}
severity_adjust other than "keep" is accepted only with an evidence entry (path:line) that justifies it.
Scoring: 0-25 inferred/pre-existing; 50 real but rare; 75 real on a real path; 90-100 reproduced or proven from definitions with no escape upstream/downstream.
reason in Korean; evidence entries "path:line — what it shows" or "<command> → <outcome>".`

// ---------------------------------------------------------------- helpers (pure code, no agents)
const bigrams = (t) => { const s = (t || '').replace(/[^\p{L}\p{N}]/gu, ''); const b = new Set(); for (let i = 0; i < s.length - 1; i++) b.add(s.slice(i, i + 2)); return b }
const similar = (a, b) => { const A = bigrams(a), B = bigrams(b); let n = 0; for (const x of A) if (B.has(x)) n++; return n / Math.max(1, Math.min(A.size, B.size)) }
const tokens = (...t) => new Set((t.join(' ').match(/[A-Za-z_][A-Za-z0-9_]{2,}|[가-힣]{2,}/g) || []).map((x) => x.toLowerCase()))
const overlap = (a, b) => { const A = tokens(...a), B = tokens(...b); if (!A.size || !B.size) return 0; let n = 0; for (const x of A) if (B.has(x)) n++; return n / Math.min(A.size, B.size) }
const SUPPRESS_EXEMPT = new Set(['security', 'data'])
const spent = () => Math.round(budget.spent() / 1000)
// agent() throws on schema retry exhaustion; a lost skeptic must become an abstain, never a silent drop.
const safe = (p) => p.then((r) => r, (e) => { log(`agent failed: ${String(e && e.message || e).slice(0, 120)}`); return null })

// Deterministic prescreen — off-hunk lines, unchanged files, and claims already refuted on the same file
// (team memory in .issueops/pr-review/refuted.jsonl; security/data are never suppressed) never reach a skeptic.
const prescreen = (c) => {
  const ranges = hunkRanges[c.path]
  if (!ranges) return `prescreen: ${c.path} is not a changed file`
  const inHunk = ranges.some(([a, b]) => c.new_line >= a && c.new_line <= b)
  if (!inHunk && !c.newly_reachable) return `prescreen: line ${c.new_line} is outside every hunk of ${c.path} and not marked newly_reachable`
  if (!SUPPRESS_EXEMPT.has(c.category)) {
    const h = refutedHistory.find((r) => r.path === c.path && overlap([r.title, r.what], [c.title, c.what]) >= 0.5)
    if (h) return `prescreen: refuted before on this file — ${(h.title || '').slice(0, 120)}`
  }
  return null
}

// ---------------------------------------------------------------- Find: first unit alone (warms the shared prompt-cache prefix), then the rest in parallel
phase('Find')
log(`${units.length} finder units (lens bundle × shard), per-lens cap ${perLensCap}${carried.length ? `, ${carried.length} findings carried from the previous head` : ''}`)
const runFinder = (u) => safe(agent(finderPrompt(u), { label: `find:${u.id}`, phase: 'Find', schema: FINDER, model: role('finder') }))
const first = units.length ? await runFinder(units[0]) : null
const rest = await parallel(units.slice(1).map((u) => () => runFinder(u)))
const finders = [first, ...rest].map((r, i) => r && { ...r, unit: units[i].id, lenses: units[i].lenses }).filter(Boolean)
const finderByUnit = new Map(finders.map((r) => [r.unit, r]))
const coverageRows = units.map((u) => {
  const expected = [...new Set(u.files || [])]
  const reviewed = finderByUnit.get(u.id)?.reviewed_files || []
  const counts = new Map()
  for (const path of reviewed) counts.set(path, (counts.get(path) || 0) + 1)
  return {
    unit: u.id,
    missing_files: expected.filter((path) => !counts.has(path)),
    duplicates: expected.filter((path) => (counts.get(path) || 0) > 1),
    unexpected: [...counts.keys()].filter((path) => !expected.includes(path)),
    expected: expected.length,
  }
})
const coverage = {
  expected_assignments: coverageRows.reduce((n, row) => n + row.expected, 0),
  covered_assignments: coverageRows.reduce((n, row) => n + row.expected - row.missing_files.length, 0),
  gaps: coverageRows.filter((row) => row.missing_files.length).map(({ unit, missing_files }) => ({ unit, missing_files })),
  duplicates: coverageRows.filter((row) => row.duplicates.length).map(({ unit, duplicates }) => ({ unit, files: duplicates })),
  unexpected: coverageRows.filter((row) => row.unexpected.length).map(({ unit, unexpected }) => ({ unit, files: unexpected })),
}
coverage.complete = coverage.gaps.length === 0 && coverage.duplicates.length === 0

const seen = new Map()
for (const r of finders) {
  for (const c of r.candidates || []) {
    const lens = c.lens || r.lenses[0]
    const key = `${c.path}:${c.new_line}:${(c.title || '').slice(0, 24)}`
    const dup = [...seen.values()].find((x) =>
      (x.path === c.path && Math.abs(x.new_line - c.new_line) <= 2 && x.category === c.category) ||
      similar(x.title, c.title) >= 0.5 || (x.rule && x.rule === c.rule && similar(x.what, c.what) >= 0.4) || (x.category === c.category && similar(x.why, c.why) >= 0.5))
    if (dup) {
      dup.lenses.push(lens); dup.confidence = Math.max(dup.confidence, c.confidence)
      if (SUPPRESS_EXEMPT.has(c.category)) dup.category = c.category   // a security/data reading must survive dedup so it is never suppressed
      dup.evidence = [...new Set([...(dup.evidence || []), ...(c.evidence || [])])]
      dup.alternates = [...(dup.alternates || []), { path: c.path, new_line: c.new_line, title: c.title }]
      continue
    }
    seen.set(key, { ...c, lenses: [lens] })
  }
}
let candidates = [...seen.values()].sort((a, b) => b.confidence - a.confidence)
const prescreened = []
candidates = candidates.filter((c) => { const why = prescreen(c); if (why) { prescreened.push({ ...c, verdicts: [{ skeptic: 'prescreen', refuted: true, confidence: 90, reason: why, evidence: [], severity_adjust: 'keep' }] }); return false } return true })
if (candidates.length > maxCandidates) { log(`dropping ${candidates.length - maxCandidates} lowest-confidence candidates (cap ${maxCandidates})`); candidates = candidates.slice(0, maxCandidates) }
log(`${finders.length}/${units.length} finders returned, coverage ${coverage.covered_assignments}/${coverage.expected_assignments}, ${seen.size} unique candidates, ${prescreened.length} removed by prescreen, ${candidates.length} to verify — ${spent()}k output tokens so far`)

// ---------------------------------------------------------------- Verify: blind tracer → reproducer only if the tracer failed to refute
phase('Verify')
const KILL = 70
const UNVERIFIED_MAX_CONFIDENCE = 40
// A structurally valid low-confidence non-refutation is an abstention, not evidence.
const isUsableVerdict = (v) => v && (v.refuted || (v.confidence > UNVERIFIED_MAX_CONFIDENCE && !v.reason.trimStart().startsWith('미확인:')))
const verified = await pipeline(
  candidates,
  (c) => safe(agent(skepticPrompt('tracer', c), { label: `verify:tracer:${c.path.split('/').pop()}:${c.new_line}`, phase: 'Verify', schema: VERDICT, model: role('tracer'), effort: 'medium' }))
    .then((t) => ({ candidate: c, tracer: t, attempted: t ? 1 : 0 })),
  (s) => {
    if (s.tracer && s.tracer.refuted && s.tracer.confidence >= KILL) return { candidate: s.candidate, tracer: s.tracer, verdicts: [s.tracer], skipped: 'reproducer' }
    return safe(agent(skepticPrompt('reproducer', s.candidate, s.tracer), { label: `verify:reproducer:${s.candidate.path.split('/').pop()}:${s.candidate.new_line}`, phase: 'Verify', schema: VERDICT, model: role('reproducer'), effort: 'medium' }))
      .then((r) => ({ candidate: s.candidate, tracer: s.tracer, attempted: s.attempted + (r ? 1 : 0),
        verdicts: [s.tracer, r].filter(isUsableVerdict) }))
  },
)

const findings = []
const refuted = [...prescreened]
let abstained = 0
let skippedReproducers = 0
const order = ['low', 'medium', 'high', 'critical']
for (const v of verified.filter(Boolean)) {
  if (v.skipped) skippedReproducers++
  const killer = v.verdicts.find((x) => x.refuted && x.confidence >= KILL)
  const keep = v.verdicts.filter((x) => !x.refuted)
  if (killer) { refuted.push({ ...v.candidate, verdicts: v.verdicts }); continue }
  if (v.verdicts.length < 2) { abstained++; log(`abstain: ${v.candidate.path}:${v.candidate.new_line} had ${v.verdicts.length} verdict(s); kept at confidence 50`); findings.push({ ...v.candidate, confidence: Math.min(50, v.candidate.confidence), verification: 'skeptics unavailable (abstain)' }); continue }
  if (keep.length < 2) { refuted.push({ ...v.candidate, verdicts: v.verdicts }); continue }
  const confidence = Math.min(v.candidate.confidence, ...keep.map((x) => x.confidence))
  // Severity moves only on a skeptic that brought evidence (models' unsupported severity scores are noise — Greptile 2025).
  const adj = keep.filter((x) => (x.evidence || []).length).map((x) => x.severity_adjust).find((s) => s !== 'keep')
  let severity = v.candidate.severity
  if (adj === 'lower') severity = order[Math.max(0, order.indexOf(severity) - 1)]
  if (adj === 'raise') severity = order[Math.min(3, order.indexOf(severity) + 1)]
  const corrected = keep.find((x) => x.corrected_line)
  const correctedSug = keep.find((x) => x.corrected_suggestion)
  findings.push({ ...v.candidate, severity, confidence, skeptics_passed: true, new_line: corrected ? corrected.corrected_line : v.candidate.new_line,
    suggestion: correctedSug ? correctedSug.corrected_suggestion : v.candidate.suggestion,
    verification: keep.flatMap((x) => x.evidence).filter((e) => e.includes('→')).slice(0, 4).join(' / ') || keep.map((x) => `${x.skeptic}: ${x.reason.slice(0, 80)}`).join(' / '), evidence: [...new Set([...(v.candidate.evidence || []), ...keep.flatMap((x) => x.evidence)])].slice(0, 10) })
}
// Partial refutations: killed candidates where a skeptic still stood with confidence ≥ 70 carry residual risk for the author.
const partial = refuted
  .map((r) => ({ path: r.path, new_line: r.new_line, title: r.title,
    killed_by: r.verdicts.filter((x) => x.refuted).map((x) => `${x.skeptic}(${x.confidence}): ${x.reason}`),
    residual: r.verdicts.filter((x) => !x.refuted && x.confidence >= KILL).map((x) => `${x.skeptic}(${x.confidence}): ${x.reason}`) }))
  .filter((r) => r.residual.length)
// Refutations worth remembering (team memory): killed by a skeptic with evidence, not by prescreen.
const refutedForHistory = refuted.filter((r) => r.verdicts.some((x) => x.refuted && x.skeptic !== 'prescreen' && x.confidence >= 80))
  .map((r) => ({ path: r.path, new_line: r.new_line, title: r.title, what: (r.what || '').slice(0, 300), category: r.category, killed_by: r.verdicts.filter((x) => x.refuted).map((x) => `${x.skeptic}(${x.confidence}): ${(x.reason || '').slice(0, 200)}`) }))
const failureCounts = {
  agent_failure: units.length - finders.length,
  low_confidence_abstain: abstained,
  coverage_gap: coverage.gaps.reduce((n, gap) => n + gap.missing_files.length, 0)
    + coverage.duplicates.reduce((n, item) => n + item.files.length, 0),
}
const status = failureCounts.agent_failure || failureCounts.low_confidence_abstain || failureCounts.coverage_gap ? 'degraded' : 'ok'
log(`${findings.length - abstained} confirmed, ${abstained} abstained (+${carried.length} carried), ${refuted.length} refuted (${prescreened.length} by prescreen, ${skippedReproducers} reproducers skipped, ${partial.length} with residual risk → open_questions, coverage ${coverage.covered_assignments}/${coverage.expected_assignments}, status=${status}) — ${spent()}k output tokens total`)
return { findings: [...findings, ...carried], refuted, partial, refuted_for_history: refutedForHistory, verified_ok: finders.flatMap((r) => r.verified_ok || []), inspected: finders.map((r) => ({ unit: r.unit, lenses: r.lenses, reviewed_files: r.reviewed_files, inspected: r.inspected })),
  cost: { finders: finders.length, skeptics: verified.filter(Boolean).reduce((n, v) => n + (v.attempted || v.verdicts.length), 0), prescreened: prescreened.length, reproducers_skipped: skippedReproducers, carried: carried.length, output_tokens_k: spent(),
    models: { finder: role('finder'), tracer: role('tracer'), reproducer: role('reproducer') } },
  coverage, status, degraded: status === 'degraded', failure_counts: failureCounts }
