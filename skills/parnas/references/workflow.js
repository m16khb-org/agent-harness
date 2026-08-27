// Parnas inspection workflow (Claude Code `Workflow` tool). Pass via `script`, with
// args = { outDir, checkout, codegraph, lenses: [...applicable lens ids], lensText: {id: text},
//          maxCandidates: 24 }
// The coordinator (you) builds the context pack BEFORE this runs and merges/posts AFTER.
export const meta = {
  name: 'parnas-inspection',
  description: 'Parallel multi-lens MR inspection with adversarial verification',
  phases: [
    { title: 'Find', detail: 'one inspector per lens, independent' },
    { title: 'Verify', detail: 'three skeptics per candidate (tracer / reproducer / scoper)' },
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
  },
  required: ['path', 'new_line', 'severity', 'category', 'title', 'what', 'why', 'how', 'evidence', 'confidence'],
}
const FINDER = {
  type: 'object',
  properties: { lens: { type: 'string' }, inspected: { type: 'array', items: { type: 'string' } },
    candidates: { type: 'array', items: CANDIDATE },
    verified_ok: { type: 'array', items: { type: 'object', properties: { concern: { type: 'string' }, why_ok: { type: 'string' }, loc: { type: 'string' }, thread: { type: ['string', 'null'] } }, required: ['concern', 'why_ok', 'loc'] } } },
  required: ['lens', 'inspected', 'candidates', 'verified_ok'],
}
const VERDICT = {
  type: 'object',
  properties: { skeptic: { type: 'string' }, refuted: { type: 'boolean' }, confidence: { type: 'integer' }, reason: { type: 'string' },
    evidence: { type: 'array', items: { type: 'string' } }, severity_adjust: { type: 'string', enum: ['keep', 'lower', 'raise'] },
    corrected_line: { type: ['integer', 'null'] }, corrected_suggestion: { type: ['string', 'null'] } },
  required: ['skeptic', 'refuted', 'confidence', 'reason', 'evidence', 'severity_adjust'],
}

const { outDir, checkout, codegraph, lenses, lensText, maxCandidates = 24, perLensCap = 6 } = args
const cg = codegraph ? '`codegraph explore "<symbol>"` prints definitions + call paths — use it before grep.' : 'Use rg and direct reads for symbol discovery.'

const finderPrompt = (id) => `You are one inspector in a formal design inspection. Lens: ${id} — ${lensText[id]}
Repository checkout at head: ${checkout} (read-only; never edit, never checkout, never post).
Context pack: ${outDir}/summary.md (read first), ${outDir}/diff.patch (the cumulative diff).
Rule pack files are listed in summary.md; read the ones whose globs match the files you inspect.
${cg}

Before inspecting, read in summary.md: "Linked issues" (what the change is supposed to do), "Existing threads" (lines that already have a bot/human thread — do not re-raise those unless you contradict them), "Prior review lessons" (claims already refuted on these files — never repeat them). If ${outDir}/gate.md exists, read it: lint/typecheck/test results and LOC metrics are already measured; never re-report what the gate reports.

Inspect ONLY through your lens. For every candidate defect you MUST, before reporting:
1. Open the real definition of every symbol the claim depends on — the file at its real path in the checkout, not the fragment in diff.patch. A diff hunk is not a definition.
2. Walk one hop upstream (who calls / what validates the input before it gets here) and one hop downstream (who consumes the result) and state what you saw. If a hop is not findable (external library, dynamic dispatch), say so explicitly in upstream/downstream and cap confidence at 50 — do not guess the hop and do not silently drop the candidate.
3. Confirm the defect is on lines this change added/modified (hunks in summary.md), or that the change makes an existing problem newly reachable.
Report a candidate only if you can state a concrete failure scenario (input/state → wrong result). Return at most ${perLensCap} candidates, strongest scenario first; confidence follows the rubric (25 inferred / 50 verified-but-rare / 75 verified on a real path / 90+ proven with no escape upstream or downstream), not your enthusiasm.
Do NOT report: style, naming, things a linter/typechecker/CI catches, pre-existing issues on untouched lines, speculative refactors, "consider adding" without a failure scenario.
verified_ok: things that looked risky and you cleared — {concern: "<우려, 한 구절>", why_ok: "<왜 괜찮은지, 한 문장, 근거 파일:라인 포함>", loc: "file:line", thread: "<existing thread id you are contradicting, else null>"}. Max 6.
suggestion is REQUIRED when category is api-contract or the fix is a decorator/description/config one-liner — the author should be able to apply it with one click.
All prose fields in Korean; identifiers/paths verbatim. evidence entries: "path:line — what it proves".`

const SKEPTIC_TEXT = {
  tracer: 'Prove the claim rests on an inferred shape or an unchecked boundary. Open the real definition of every symbol in the claim. Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto/schema constraints, caller preconditions) and downstream to the consumer of the result. Report each hop as path:line. If any hop neutralises the scenario, refute.',
  reproducer: 'Prove the failure scenario cannot actually happen. Try to make it happen: write a throwaway unit test in the checkout encoding the scenario and run it, or run the targeted typecheck/lint on the file, or execute a small script. Paste command and outcome. Delete throwaway files afterwards. A scenario that cannot be reproduced and cannot be argued from definitions is refuted.',
  scoper: 'Prove this is not this change\'s problem. Check the cumulative diff hunks: is the defect on added/modified lines or newly reachable because of them? Check git log -L / git blame of the lines: pre-existing? Check the description, the linked issue in summary.md, and the other changes: is the behavior intentional AND correct with respect to what the issue asks? (Intentional but wrong per the issue is still a defect — do not refute on intent alone.) Check prior_review_lessons in context.json: was this exact claim refuted before? Refute if pre-existing, intentional-and-correct, or already refuted.',
}

const skepticPrompt = (id, c) => `You are a skeptic in a design inspection. Your job is to try to REFUTE this candidate defect. Lens: ${id} — ${SKEPTIC_TEXT[id]}
refuted=true ONLY when you actually neutralised the scenario with evidence (a definition, a boundary, a run that shows it cannot happen, proof it is pre-existing/already refuted). If you could not test it (no runnable environment, missing DB, tooling absent) or could not find the hop, return refuted=false with confidence ≤ 40 and reason starting with "미확인:" — inability to verify is not a refutation.
Checkout (read-only except throwaway test files you delete afterwards): ${checkout}
Context pack: ${outDir}/summary.md, ${outDir}/diff.patch, ${outDir}/context.json
Candidate: ${JSON.stringify(c)}
Scoring: 0-25 inferred/pre-existing; 50 real but rare; 75 real on a real path; 90-100 reproduced or proven from definitions with no escape upstream/downstream.
reason in Korean; evidence entries "path:line — what it shows" or "<command> → <outcome>".`

phase('Find')
const finderResults = await parallel(lenses.map((id) => () => agent(finderPrompt(id), { label: `find:${id}`, phase: 'Find', schema: FINDER })))
const finders = finderResults.filter(Boolean)
const bigrams = (t) => { const s = (t || '').replace(/[^\p{L}\p{N}]/gu, ''); const b = new Set(); for (let i = 0; i < s.length - 1; i++) b.add(s.slice(i, i + 2)); return b }
const similar = (a, b) => { const A = bigrams(a), B = bigrams(b); let n = 0; for (const x of A) if (B.has(x)) n++; return n / Math.max(1, Math.min(A.size, B.size)) }
const seen = new Map()
for (const r of finders) {
  for (const c of r.candidates || []) {
    const key = `${c.path}:${c.new_line}:${(c.title || '').slice(0, 24)}`
    const dup = [...seen.values()].find((x) =>
      (x.path === c.path && Math.abs(x.new_line - c.new_line) <= 2 && x.category === c.category) ||
      similar(x.title, c.title) >= 0.5 || (x.rule && x.rule === c.rule && similar(x.what, c.what) >= 0.4) || (x.category === c.category && similar(x.why, c.why) >= 0.5))
    if (dup) {
      dup.lenses.push(r.lens); dup.confidence = Math.max(dup.confidence, c.confidence)
      dup.evidence = [...new Set([...(dup.evidence || []), ...(c.evidence || [])])]
      dup.alternates = [...(dup.alternates || []), { path: c.path, new_line: c.new_line, title: c.title }]
      continue
    }
    seen.set(key, { ...c, lenses: [r.lens] })
  }
}
let candidates = [...seen.values()].sort((a, b) => b.confidence - a.confidence)
if (candidates.length > maxCandidates) { log(`dropping ${candidates.length - maxCandidates} lowest-confidence candidates (cap ${maxCandidates})`); candidates = candidates.slice(0, maxCandidates) }
log(`${finders.length}/${lenses.length} finders returned, ${candidates.length} unique candidates`)

phase('Verify')
const verified = await pipeline(
  candidates,
  (c) => parallel(['tracer', 'reproducer', 'scoper'].map((id) => () =>
    agent(skepticPrompt(id, c), { label: `verify:${id}:${c.path.split('/').pop()}:${c.new_line}`, phase: 'Verify', schema: VERDICT })))
    .then((vs) => ({ candidate: c, verdicts: vs.filter(Boolean) })),
)

const findings = []
const refuted = []
for (const v of verified.filter(Boolean)) {
  const by = Object.fromEntries(v.verdicts.map((x) => [x.skeptic, x]))
  const killer = v.verdicts.find((x) => x.refuted && x.confidence >= 70)
  const keep = v.verdicts.filter((x) => !x.refuted)
  if (v.verdicts.length < 2) { log(`abstain: ${v.candidate.path}:${v.candidate.new_line} had ${v.verdicts.length} verdict(s); kept at confidence 50`); findings.push({ ...v.candidate, confidence: Math.min(50, v.candidate.confidence), verification: 'skeptics unavailable (abstain)' }); continue }
  if (killer || keep.length < 2) { refuted.push({ ...v.candidate, verdicts: v.verdicts }); continue }
  const confidence = Math.min(v.candidate.confidence, ...keep.map((x) => x.confidence))
  const adj = keep.map((x) => x.severity_adjust).find((s) => s !== 'keep')
  const order = ['low', 'medium', 'high', 'critical']
  let severity = v.candidate.severity
  if (adj === 'lower') severity = order[Math.max(0, order.indexOf(severity) - 1)]
  if (adj === 'raise') severity = order[Math.min(3, order.indexOf(severity) + 1)]
  const corrected = keep.find((x) => x.corrected_line)
  const correctedSug = keep.find((x) => x.corrected_suggestion)
  findings.push({ ...v.candidate, severity, confidence, skeptics_passed: keep.length === v.verdicts.length, new_line: corrected ? corrected.corrected_line : v.candidate.new_line,
    suggestion: correctedSug ? correctedSug.corrected_suggestion : v.candidate.suggestion,
    verification: keep.flatMap((x) => x.evidence).filter((e) => e.includes('→')).slice(0, 4).join(' / ') || keep.map((x) => `${x.skeptic}: ${x.reason.slice(0, 80)}`).join(' / '), evidence: [...new Set([...(v.candidate.evidence || []), ...keep.flatMap((x) => x.evidence)])].slice(0, 10) })
}
// Partial refutations: killed candidates where a skeptic still stood with confidence ≥ 70 carry residual risk for the author.
const partial = refuted
  .map((r) => ({ path: r.path, new_line: r.new_line, title: r.title,
    killed_by: r.verdicts.filter((x) => x.refuted).map((x) => `${x.skeptic}(${x.confidence}): ${x.reason}`),
    residual: r.verdicts.filter((x) => !x.refuted && x.confidence >= 70).map((x) => `${x.skeptic}(${x.confidence}): ${x.reason}`) }))
  .filter((r) => r.residual.length)
log(`${findings.length} confirmed, ${refuted.length} refuted (${partial.length} with residual risk → open_questions)`)
return { findings, refuted, partial, verified_ok: finders.flatMap((r) => r.verified_ok || []), inspected: finders.map((r) => ({ lens: r.lens, inspected: r.inspected })) }
