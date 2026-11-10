/**
 * Whether a final reply leaves something for the person to answer in prose,
 * and what to tell Claude when it does.
 *
 * Two stages: `Reply.mayAwaitUser` is a free regex screen that passes most
 * replies untouched; only a reply it flags goes to the model, whose JSON
 * answer `Verdict.parse` reads.
 */

const FENCED_CODE = /```[\s\S]*?```/g
const INLINE_CODE = /`[^`\n]*`/g

/** A question mark closing a sentence, not one inside a URL's query. */
const QUESTION = /\?(?=[\s)"'*_]|$)/m

const CUES = new RegExp(
  [
    'let me know',
    'want me to',
    'should i',
    'shall i',
    'would you like',
    'do you want',
    "if you'?d like",
    'happy to',
    'i can also',
    'your call',
    'up to you',
    'next steps?',
    'follow[- ]ups?',
    'decide',
    'decision',
    'confirm',
    'approve',
    'prefer',
    'options?:',
    'which (one|option|approach)',
  ]
    .map((cue) => `\\b${cue}\\b`)
    .join('|'),
  'i',
)

export const Reply = {
  /** Prose alone: code blocks and spans hold `?` and cue words that ask nothing. */
  prose: (text: string): string => text.replace(FENCED_CODE, '').replace(INLINE_CODE, ''),

  /** True when the reply might ask something of the person and is worth a model call. */
  mayAwaitUser: (text: string): boolean => {
    const prose = Reply.prose(text)
    return QUESTION.test(prose) || CUES.test(prose)
  },
} as const

export type Verdict = { kind: 'done' } | { kind: 'waiting'; items: readonly string[] }

const isVerdictJson = (value: unknown): value is { waiting: boolean; items?: unknown } =>
  typeof value === 'object' && value !== null && 'waiting' in value && typeof value.waiting === 'boolean'

export const Verdict = {
  /** The reviewer's instructions; the reply goes in the user message as data. */
  SYSTEM: [
    'You review the final message an AI coding agent wrote before ending its turn.',
    'Decide whether the message leaves anything waiting on the user that the agent should have asked with its AskUserQuestion tool instead of writing it in prose.',
    '',
    'Counts as waiting:',
    '- A question to the user, direct or rhetorical-but-expecting-an-answer.',
    '- A decision or choice left for the user (options, trade-offs to pick between, "your call").',
    '- An offer of further work ("Want me to...", "I can also...", "Let me know if...").',
    '- A "next steps" or "follow-ups" list the user is expected to approve, pick from, or act on.',
    '',
    'Does not count:',
    '- Reporting what was done, test results, or what was not verified and why.',
    '- An FYI caveat or limitation that needs no answer.',
    '- Instructions the user asked for (how to run something, a command to type).',
    '- Answering a question the user asked.',
    '',
    'Reply with JSON only, no prose and no code fence:',
    '{"waiting": true, "items": ["<each thing left for the user, quoted or paraphrased in under 15 words>"]}',
    'or {"waiting": false}',
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
    if (!value.waiting) return { kind: 'done' }
    const items = Array.isArray(value.items) ? value.items.filter((item): item is string => typeof item === 'string') : []
    return { kind: 'waiting', items }
  },

  /** The block reason Claude reads as its next instruction. */
  blockReason: (items: readonly string[]): string =>
    [
      'Your final message leaves something for the user to answer in prose:',
      ...(items.length > 0 ? items.map((item) => `- ${item}`) : ['- (a question, decision or offer)']),
      '',
      'Ask it with the AskUserQuestion tool instead: one question per decision, concrete options, your recommendation first.',
      'An offer of more work becomes a question with yes/no options. Then act on the answer.',
      'Keep any summary to outcomes; do not repeat the questions in prose.',
      'If nothing actually needs the user, end the turn without asking.',
    ].join('\n'),
} as const
