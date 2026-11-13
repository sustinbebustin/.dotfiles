/**
 * Whether a final reply ends the turn with something still open, and the
 * prompt that sends Claude back to it.
 *
 * The reviewer model lists what the reply leaves pending and labels each item
 * by kind; the code, not the model, decides which kinds warrant a nudge. A
 * small model asked "is anything open?" says yes to in-flight work it has just
 * called unactionable; asked to label, it separates the two. The criteria come
 * from mined sessions where the person had to ask "what's next?" or "use
 * AskUserQuestion": outcome reports whose tail ("Still to do", "Open for you",
 * "Say the word and I'll...") was left in prose.
 */

/** What one pending thing in a reply is; only `ask`, `offer` and `todo` warrant a nudge. */
export type ItemKind = 'ask' | 'offer' | 'todo' | 'running' | 'later' | 'fyi'

const ITEM_KINDS: readonly ItemKind[] = ['ask', 'offer', 'todo', 'running', 'later', 'fyi']
const NUDGE_KINDS: ReadonlySet<ItemKind> = new Set(['ask', 'offer', 'todo'])

export type Item = { what: string; kind: ItemKind }

export type Verdict = { kind: 'done' } | { kind: 'open'; items: readonly string[] }

const isItemKind = (value: unknown): value is ItemKind => ITEM_KINDS.some((kind) => kind === value)

const isItem = (value: unknown): value is Item =>
  typeof value === 'object' && value !== null && 'what' in value && typeof value.what === 'string' && 'kind' in value && isItemKind(value.kind)

const isVerdictJson = (value: unknown): value is { items: unknown[] } =>
  typeof value === 'object' && value !== null && 'items' in value && Array.isArray(value.items)

/** Leads every nudge, so the turn it starts is recognised and not reviewed again. */
const NUDGE_HEADER = 'ask-dont-tell review:'

export const Verdict = {
  NUDGE_HEADER,

  /** The reviewer's instructions; the reply goes in the user message as data. */
  SYSTEM: [
    'You review the final message an AI coding agent wrote before ending its turn and handing control back to the user.',
    'List everything the message leaves pending, and label each item with exactly one kind. List only what the message states; an item you would have to infer is not in it.',
    '',
    '- "ask": a question or decision for the user, in any form: "Tell me which you want next", options or trade-offs to pick from, "your call", "Decision for you".',
    '- "offer": further work offered or recommended, awaiting a yes: "Want me to...", "I can also...", "Say the word and I\'ll...", "Should I...", "I\'d add a check", "a one-line PR whenever you want it".',
    '- "todo": unfinished work that the agent or the user could act on right now and nobody is doing: "Still to do" items, uncommitted or unpushed changes, a branch with no PR, an untriaged ticket, CI the agent just triggered and has not checked.',
    '- "running": work already in flight that will finish or report back by itself: a background job, subagent, other session, CI or test run, a watch ("6 of 7 done, waiting on its test run, about an hour left"; "the watch is re-armed"; "checking whether a parallel walk helps").',
    '- "later": can only happen later or is blocked on something nobody here can do now: a future date, a third party, measuring after a deploy; or a state the user chose ("still unpushed, as you asked").',
    '- "fyi": a caveat, limitation, side effect, or something left unverified that needs no answer.',
    '',
    'Not pending at all, so not listed: what was done, test results, an answer to the user\'s question that leaves nothing to do, commands the user asked for or that only the user can run.',
    'A status report on running work, even one answering the user\'s question, lists that work as "running", not "todo".',
    '',
    'Reply with JSON only, no prose and no code fence:',
    '{"items": [{"what": "<the item, quoted or paraphrased in under 15 words>", "kind": "<kind>"}]}',
    'or {"items": []} when nothing is pending.',
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
    const open = value.items.filter(isItem).filter((item) => NUDGE_KINDS.has(item.kind))
    return open.length === 0 ? { kind: 'done' } : { kind: 'open', items: open.map((item) => item.what) }
  },

  /** Whether a turn's prompt is one of this mod's nudges. */
  isNudge: (prompt: string): boolean => prompt.includes(NUDGE_HEADER),

  /** The prompt that sends Claude back to what its reply left open. */
  nudge: (items: readonly string[]): string =>
    [
      `${NUDGE_HEADER} your last reply ended the turn with work or decisions still open:`,
      ...items.map((item) => `- ${item}`),
      '',
      'For each item: if it is within what the user asked, reversible, and you can do it now, do it.',
      'Otherwise ask with the AskUserQuestion tool: a choice between approaches; anything outward-facing or irreversible (push, merge, release, deploy, deleting shared state); new scope such as a next phase.',
      'One question per decision, concrete options, your recommendation first. An offer of more work becomes a yes/no question.',
      'Keep any summary to outcomes; do not repeat the questions in prose.',
      'If nothing is actually open, say so in one line and stop.',
    ].join('\n'),
} as const
