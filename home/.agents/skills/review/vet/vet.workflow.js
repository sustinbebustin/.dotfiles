export const meta = {
  name: 'vet',
  description: 'Review a scoped diff through lens-picked reviewers, then adversarially verify their findings',
  whenToUse: 'Dispatched by the /vet skill with args built from its scope block; not for direct use.',
  phases: [
    { title: 'Review', detail: 'one agent per reviewer instance, each on its lens files' },
    { title: 'Verify', detail: 'refute or confirm blocking and medium-confidence findings, sweep variants' },
  ],
}

// The finding contract. This file is its single source of truth; contract.md explains the
// fields and how the orchestrator routes them.
const CATEGORIES = [
  // substantive
  'logic', 'error-handling', 'concurrency', 'security', 'authz', 'data', 'contract', 'deploy',
  'devex', 'caching', 'react-purity', 'performance',
  // quality and the rest
  'type-safety', 'structure', 'duplication', 'naming', 'tests', 'comments', 'docs', 'a11y', 'ux',
  'design-system', 'shell',
]
const SUBSTANTIVE = new Set(CATEGORIES.slice(0, 12))
const str = { type: 'string' }
const strs = { type: 'array', items: str }
const REACHABILITY = {
  type: 'object',
  properties: {
    status: { enum: ['verified', 'unverified-cross-boundary', 'gated'] },
    entry_points: strs,
    notes: str,
  },
  required: ['status', 'entry_points', 'notes'],
}
const FINDING = {
  type: 'object',
  properties: {
    id: str,
    file: { type: 'string', description: 'workspace-relative path' },
    line: { type: 'integer', description: 'line number in the file as it stands after the change, never an offset into the diff' },
    end_line: { type: 'integer' },
    summary: { type: 'string', description: 'one sentence: what is wrong' },
    failure_scenario: { type: 'string', description: 'concrete inputs or state -> wrong outcome' },
    evidence: { type: 'string', description: 'what you read: lines, symbols, search results' },
    category: { enum: CATEGORIES },
    blocking: { type: 'boolean' },
    confidence: { enum: ['high', 'medium', 'low'] },
    business_risk: { enum: ['none', 'possible', 'likely'] },
    reachability: REACHABILITY,
    proposed_fix: str,
    fix_kind: { enum: ['mechanical', 'behavioral', 'design', 'docs'] },
  },
  required: ['id', 'file', 'line', 'summary', 'failure_scenario', 'evidence', 'category', 'blocking',
    'confidence', 'business_risk', 'proposed_fix', 'fix_kind'],
}
const REVIEW_SCHEMA = {
  type: 'object',
  properties: {
    lens: str,
    files_read: strs,
    findings: { type: 'array', items: FINDING },
    open_questions: {
      type: 'array',
      items: {
        type: 'object',
        properties: { id: str, question: str, seen_at: str, why_asking: str },
        required: ['id', 'question', 'seen_at', 'why_asking'],
      },
    },
    passed_checks: strs,
    previous_findings_status: {
      type: 'array',
      items: {
        type: 'object',
        properties: { id: str, status: { enum: ['FIXED', 'STILL_PRESENT', 'WONTFIX'] }, note: str },
        required: ['id', 'status', 'note'],
      },
    },
  },
  required: ['lens', 'files_read', 'findings', 'open_questions', 'passed_checks'],
}
const VERDICT = {
  type: 'object',
  properties: {
    id: str,
    verdict: { enum: ['CONFIRMED', 'PLAUSIBLE', 'REFUTED'] },
    reason: str,
    evidence: str,
    reachability: REACHABILITY,
    variants: {
      type: 'array',
      items: {
        type: 'object',
        properties: { file: str, line: { type: 'integer' }, note: str },
        required: ['file', 'line', 'note'],
      },
    },
  },
  required: ['id', 'verdict', 'reason', 'evidence', 'reachability', 'variants'],
}
const VERIFY_SCHEMA = {
  type: 'object',
  properties: { verdicts: { type: 'array', items: VERDICT } },
  required: ['verdicts'],
}

const MAX_AGENTS = 9
const AGENTS = ['vet-reviewer', 'doc-comment-reviewer', 'design-consultant']

// --- args ---------------------------------------------------------------------
// {
//   briefing: absolute path to the briefing file,
//   lens_dir: absolute path to the lenses directory,
//   reviewers: [{ label, agent, lenses: [lens names], files?: [paths], note?: string }],
//   verify: { slots: n, inline_max: n, batched: bool },
// }
function bad(what) {
  throw new Error(`vet.workflow args: ${what}. Nothing was dispatched; fix the args the /vet skill built and re-run.`)
}
const A = args || bad('missing; pass the args object')
if (!A.briefing) bad('`briefing` (path to the briefing file) is required')
if (!A.lens_dir) bad('`lens_dir` (path to lenses/) is required')
if (!Array.isArray(A.reviewers) || A.reviewers.length === 0) bad('`reviewers` must be a non-empty array')
for (const r of A.reviewers) {
  if (!r.label) bad('every reviewer needs a `label`')
  if (!AGENTS.includes(r.agent)) bad(`reviewer ${r.label}: agent must be one of ${AGENTS.join(', ')}, got ${r.agent}`)
  if (r.agent === 'vet-reviewer' && !(Array.isArray(r.lenses) && r.lenses.length)) bad(`reviewer ${r.label}: lenses required`)
}
const V = A.verify || { slots: 1, inline_max: 0, batched: false }
if (A.reviewers.length + V.slots > MAX_AGENTS) {
  bad(`${A.reviewers.length} reviewers + ${V.slots} verifier slots exceeds the ${MAX_AGENTS}-agent cap; attach lenses to fewer instances or chunk the run`)
}

// --- prompts --------------------------------------------------------------------
function reviewPrompt(r) {
  const files = r.files && r.files.length
    ? `\nFiles assigned to you (a parallel reviewer covers the rest of the scope; a path marked "(deleted)" is gone from the change: read it at the base with \`git show <base sha>:<path>\` and review what its removal breaks):\n${r.files.map(f => `- ${f}`).join('\n')}\n`
    : ''
  const note = r.note ? `\n${r.note}\n` : ''
  if (r.agent === 'doc-comment-reviewer') {
    return `Review the comments and doc comments added or edited in the change described in the briefing at ${A.briefing}. Read it first and run its diff commands verbatim.
${files}${note}
A comment that cites an id, path, or note living only in a gitignored or scratch location (.scratch/, .claude/logs/, a plan file, a session or ticket id with no durable home) is not durable context: grade it as session-scoped commentary.

Set lens to "comments" and category to "comments" on every finding.`
  }
  if (r.agent === 'design-consultant') {
    return `Review mode. The change is described in the briefing at ${A.briefing}; read it first and run its diff commands verbatim.
${files}${note}
Set lens to "design". Use the impeccable references for depth, not their report format, and never run an ask-the-user flow.`
  }
  const lensFiles = r.lenses.map(l => `- ${A.lens_dir}/${l}.md`).join('\n')
  return `Review the change described in the briefing at ${A.briefing}. Read it first: its diff commands are the only correct ones.

Lenses -- read each file before reviewing and apply every target in it:
${lensFiles}
${files}${note}
Set lens to "${r.lenses.join('+')}".`
}

function verifyPrompt(findings, label) {
  return `Verify these findings from the ${label} review of the change described in the briefing at ${A.briefing}. Read the briefing first; it names the scoped repos, the sibling repos for the variant sweep, and any data probes.

Return exactly one verdict per id below.

${JSON.stringify(findings, null, 2)}`
}

// --- stages ---------------------------------------------------------------------
// agent() returns null when an agent dies and throws when it cannot start (an unknown agent
// type); both come back as a value, so a pipeline never drops a reviewer without a trace.
async function tryAgent(prompt, opts) {
  try {
    const out = await agent(prompt, opts)
    return out ? { out } : { error: 'agent returned nothing (skipped or died after retries)' }
  } catch (e) {
    return { error: String((e && e.message) || e) }
  }
}

async function review(r) {
  const { out, error } = await tryAgent(reviewPrompt(r), {
    label: `review:${r.label}`, phase: 'Review', agentType: r.agent, schema: REVIEW_SCHEMA,
  })
  if (!out) return { reviewer: r, failed: true, error, findings: [], open_questions: [], passed_checks: [] }
  // A blocking finding without reachability goes to the verifier as unverified, not as verified.
  const findings = out.findings.map(f => ({
    ...f,
    id: `${r.label}:${f.id}`,
    reachability: f.reachability || (f.blocking
      ? { status: 'unverified-cross-boundary', entry_points: [], notes: 'reviewer gave no reachability' }
      : undefined),
    sources: [r.label],
  }))
  return { reviewer: r, failed: false, ...out, findings }
}

const needsVerify = f =>
  f.blocking
  || (f.confidence === 'medium' && SUBSTANTIVE.has(f.category))
  || (f.reachability && f.reachability.status === 'unverified-cross-boundary')

let slotsLeft = V.slots
const capped = []

const inline = batch => batch.map(f => ({ ...f, verdict: null, verify_note: 'inline: orchestrator verifies' }))

// Verify one batch of findings; findings without a verdict come back marked for the orchestrator.
async function verifyBatch(batch, label) {
  if (slotsLeft <= 0) {
    capped.push(...batch.map(f => f.id))
    return batch.map(f => ({ ...f, verdict: null, verify_note: 'verifier cap reached: orchestrator verifies' }))
  }
  slotsLeft--
  const { out: res, error } = await tryAgent(verifyPrompt(batch, label), {
    label: `verify:${label}`, phase: 'Verify', agentType: 'vet-verifier', schema: VERIFY_SCHEMA,
  })
  const byId = new Map((res ? res.verdicts : []).map(v => [v.id, v]))
  return batch.map(f => {
    const v = byId.get(f.id)
    if (!v) return { ...f, verdict: null, verify_note: res ? 'verifier returned no verdict for this id' : `verifier failed: ${error}` }
    return {
      ...f,
      verdict: v.verdict,
      verify_note: v.reason,
      verify_evidence: v.evidence,
      reachability: v.reachability || f.reachability,
      variants: v.variants,
    }
  })
}

async function verifyReview(rev) {
  const want = rev.findings.filter(needsVerify)
  if (!want.length) return rev
  const verified = want.length <= V.inline_max ? inline(want) : await verifyBatch(want, rev.reviewer.label)
  const byId = new Map(verified.map(f => [f.id, f]))
  return { ...rev, findings: rev.findings.map(f => byId.get(f.id) || f) }
}

// --- merge ----------------------------------------------------------------------
const RANK = { high: 2, medium: 1, low: 0 }
const weight = f => (f.blocking ? 10 : 0) + RANK[f.confidence]
const sameDefect = (a, b) => a.file === b.file && a.category === b.category && Math.abs(a.line - b.line) <= 3

// Two verdicts on one merged finding: any REFUTED against a CONFIRMED is a conflict -> PLAUSIBLE.
function mergeVerdict(a, b) {
  if (!a) return b
  if (!b) return a
  if (a === b) return a
  if (a === 'REFUTED' || b === 'REFUTED') return 'PLAUSIBLE'
  return a === 'CONFIRMED' || b === 'CONFIRMED' ? 'CONFIRMED' : 'PLAUSIBLE'
}

function dedupe(findings) {
  const out = []
  for (const f of findings) {
    const hit = out.find(o => sameDefect(o, f))
    if (!hit) { out.push({ ...f, corroborated_by: [] }); continue }
    const keep = weight(f) > weight(hit) ? { ...f, corroborated_by: hit.corroborated_by } : hit
    keep.sources = [...new Set([...hit.sources, ...f.sources])]
    keep.corroborated_by = keep.sources.slice(1)
    keep.verdict = mergeVerdict(hit.verdict, f.verdict)
    if (hit.verify_note && f.verify_note && hit.verify_note !== f.verify_note) {
      keep.verify_note = `${hit.verify_note} | ${f.verify_note}`
    }
    keep.variants = [...(hit.variants || []), ...(f.variants || [])]
    out[out.indexOf(hit)] = keep
  }
  return out
}

// Worst status wins when reviewers disagree about a prior finding.
function mergePrior(reviews) {
  const order = { STILL_PRESENT: 2, WONTFIX: 1, FIXED: 0 }
  const m = new Map()
  for (const r of reviews) for (const p of r.previous_findings_status || []) {
    const cur = m.get(p.id)
    if (!cur || order[p.status] > order[cur.status]) m.set(p.id, { ...p, by: r.reviewer.label })
  }
  return [...m.values()]
}

// --- run ------------------------------------------------------------------------
let reviews
let findings
if (V.batched) {
  // Deep tier: dedupe across every reviewer before spending the verifier slots on batches.
  phase('Review')
  reviews = (await parallel(A.reviewers.map(r => () => review(r)))).filter(Boolean)
  const merged = dedupe(reviews.flatMap(r => r.findings))
  const want = merged.filter(needsVerify)
  let verified = []
  if (want.length <= V.inline_max) {
    verified = inline(want)
  } else if (want.length) {
    phase('Verify')
    // Keep one file's findings in one batch so a verifier traces each call path once.
    const files = [...new Set(want.map(f => f.file))]
    const n = Math.max(1, Math.min(V.slots, files.length))
    const batches = Array.from({ length: n }, () => [])
    files.forEach((file, i) => batches[i % n].push(...want.filter(f => f.file === file)))
    verified = (await parallel(batches.map((b, i) => () => verifyBatch(b, `batch-${i + 1}`)))).filter(Boolean).flat()
  }
  const byId = new Map(verified.map(f => [f.id, f]))
  findings = merged.map(f => byId.get(f.id) || f)
} else {
  reviews = (await pipeline(A.reviewers, r => review(r), rev => verifyReview(rev))).filter(Boolean)
  findings = dedupe(reviews.flatMap(r => r.findings))
}

if (capped.length) log(`verifier cap reached: ${capped.length} finding(s) left for the orchestrator to verify inline`)
const failed = reviews.filter(r => r.failed).map(r => `${r.reviewer.label} (${r.error})`)
if (failed.length) log(`reviewer(s) failed: ${failed.join('; ')}`)

return {
  reviewers: reviews.map(r => ({
    label: r.reviewer.label,
    agent: r.reviewer.agent,
    lenses: r.reviewer.lenses || [],
    status: r.failed ? 'failed' : 'ok',
    error: r.error,
    files_read: r.files_read || [],
    passed_checks: r.passed_checks || [],
    blocking: r.findings.filter(f => f.blocking).length,
    non_blocking: r.findings.filter(f => !f.blocking).length,
  })),
  findings: findings.filter(f => f.verdict !== 'REFUTED'),
  refuted: findings.filter(f => f.verdict === 'REFUTED'),
  open_questions: reviews.flatMap(r => (r.open_questions || []).map(q => ({ ...q, source: r.reviewer.label }))),
  previous_findings_status: mergePrior(reviews),
  unverified_ids: findings.filter(f => needsVerify(f) && !f.verdict).map(f => f.id),
  agents_used: reviews.length + (V.slots - slotsLeft),
}
