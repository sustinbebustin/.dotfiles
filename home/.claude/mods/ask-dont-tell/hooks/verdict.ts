/**
 * Whether a final reply ends the turn with something still open, and the
 * prompt that sends Claude back to it.
 *
 * The reviewer model reads every reply; its JSON answer is `Verdict.parse`'s
 * input. The criteria come from mined sessions where the person had to ask
 * "what's next?" or "use AskUserQuestion": long outcome reports whose tail
 * ("Still to do", "Open for you", "Say the word and I'll...") was left in prose.
 */

export type Verdict = { kind: 'done' } | { kind: 'open'; items: readonly string[] }

const isVerdictJson = (value: unknown): value is { open: boolean; items?: unknown } =>
  typeof value === 'object' && value !== null && 'open' in value && typeof value.open === 'boolean'

/** Leads every nudge, so the turn it starts is recognised and not reviewed again. */
const NUDGE_HEADER = 'ask-dont-tell review:'

export const Verdict = {
  NUDGE_HEADER,

  /** The reviewer's instructions; the reply goes in the user message as data. */
  SYSTEM: [
    'You review the final message an AI coding agent wrote before ending its turn and handing control back to the user.',
    'Decide whether the turn ended with anything still open: work the agent could still do, or something waiting on the user that the agent should have asked with its AskUserQuestion tool instead of writing in prose.',
    '',
    'Counts as open:',
    '- A question to the user, in any form, including "Tell me which you want next".',
    '- A decision or choice left for the user: options, trade-offs, "your call", "Decision for you", "the remaining decisions are yours".',
    '- An offer of further work, including ones phrased as statements: "Want me to...", "I can also...", "If you want, I can run it", "Say the word and I\'ll...", "Turning it on is a one-line PR whenever you want it".',
    '- A list of remaining or pending work: "Still to do", "Still open", "Open for you", "Waiting on you", "Next steps", "loose ends", "the next phase", even when called "none blocking" or "whenever you choose".',
    '- Changes left unfinished: uncommitted, unpushed, unmerged or unreleased work, a branch with no PR, tickets still open.',
    '- A recommendation the user is expected to approve ("I\'d add a header check", "I recommend doing X").',
    '',
    'Does not count:',
    '- Reporting what was done, test results, or what was not verified and why.',
    '- An FYI caveat, limitation or side effect of finished work that needs no answer and implies no further work ("this file is shared by every copy").',
    '- Something left unverified, unless the agent could verify it now (checking CI it just triggered counts as open).',
    '- A follow-up that can only happen later or is blocked on something neither the agent nor the user can do now: a future date, a third party, measuring after a deploy.',
    '- A state the user chose ("still unpushed, as you asked").',
    '- Instructions or commands the user asked for, or a command only the user can run (an interactive login).',
    '- Answering a question the user asked, when the answer leaves nothing to do.',
    '',
    'Reply with JSON only, no prose and no code fence:',
    '{"open": true, "items": ["<each open thing, quoted or paraphrased in under 15 words>"]}',
    'or {"open": false}',
  ].join('\n'),

  /** The model's reply read as a verdict; `undefined` when it is not the JSON asked for. */
  parse: (text: string): Verdict | undefined => {
    let value: unknown
    try {
      value = JSON.parse(text.trim().replace(/^```(?:json)?\s*|\s*```$/g, ''))
    } catch {
      return undefined
    }
    if (!isVerdictJson(value)) return undefined
    if (!value.open) return { kind: 'done' }
    const items = Array.isArray(value.items) ? value.items.filter((item): item is string => typeof item === 'string') : []
    return { kind: 'open', items }
  },

  /** Whether a turn's prompt is one of this mod's nudges. */
  isNudge: (prompt: string): boolean => prompt.includes(NUDGE_HEADER),

  /** The prompt that sends Claude back to what its reply left open. */
  nudge: (items: readonly string[]): string =>
    [
      `${NUDGE_HEADER} your last reply ended the turn with work or decisions still open:`,
      ...(items.length > 0 ? items.map((item) => `- ${item}`) : ['- (a question, decision, offer or unfinished work)']),
      '',
      'For each item: if it is within what the user asked, reversible, and you can do it now, do it.',
      'Otherwise ask with the AskUserQuestion tool: a choice between approaches; anything outward-facing or irreversible (push, merge, release, deploy, deleting shared state); new scope such as a next phase.',
      'One question per decision, concrete options, your recommendation first. An offer of more work becomes a yes/no question.',
      'Keep any summary to outcomes; do not repeat the questions in prose.',
      'If nothing is actually open, say so in one line and stop.',
    ].join('\n'),
} as const
