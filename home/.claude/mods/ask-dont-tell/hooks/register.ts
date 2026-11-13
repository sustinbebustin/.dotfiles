/**
 * Ask, Don't Tell: when a turn ends on a reply that leaves a question, a
 * decision, an offer or unfinished work in prose, send Claude back to it: do
 * what it can, and ask the rest with AskUserQuestion.
 *
 * Hooks `turn.complete`, not `classic.Stop`: where the built-in guard
 * (cc-plugin-sec-default) loads, a user mod's `classic.Stop` hook never runs.
 * The turn has already ended by then, so the nudge is a follow-up prompt from
 * `$.prompt.submit`, not a blocked stop.
 *
 * Every main-loop reply is reviewed by a small model. One nudge per prompt the
 * person typed: a turn the nudge started is not reviewed, so a reply the
 * reviewer keeps misjudging cannot loop. A nudge is dropped when the person
 * has started typing or a new turn began while the review ran.
 */

import type { Register } from 'claude-code'
import { Verdict } from './verdict'

const MODEL = 'haiku'
const REVIEW_TIMEOUT_MS = 15_000
/** The person's prompt is context for the reviewer, not the subject; a long paste is cut. */
const PROMPT_CONTEXT_MAX = 2_000

type Turn = { id: string; prompt: string }

export const register: Register = (on) => {
  /** AskUserQuestion needs a person at the prompt; `claude -p` and SDK hosts have none. */
  let isInteractive = false
  /** The main-loop turn running now; `undefined` while the session is idle. */
  let running: Turn | undefined

  on('session.start', async ($, e, next) => {
    isInteractive = e.isInteractive
    return next(e)
  })

  on('turn.start', async ($, e, next) => {
    if (e.agentId === undefined) running = { id: e.turnId, prompt: e.text }
    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    const result = await next(e)
    if (e.agentId !== undefined) return result
    const turn = running?.id === e.turnId ? running : undefined
    running = undefined
    if (!isInteractive || e.reason !== 'answer' || e.answer.trim() === '') return result
    if (turn !== undefined && Verdict.isNudge(turn.prompt)) return result

    const context = turn === undefined || turn.prompt === '' ? '' : `<user_prompt>\n${turn.prompt.slice(0, PROMPT_CONTEXT_MAX)}\n</user_prompt>\n`
    const reply = await $.model.complete(
      {
        model: MODEL,
        system: Verdict.SYSTEM,
        prompt: `${context}<final_message>\n${e.answer}\n</final_message>`,
        maxTokens: 400,
        effort: 'low',
        timeoutMs: REVIEW_TIMEOUT_MS,
      },
      { signal: next.signal },
    )
    if (!reply.isAnswered) {
      // Abandoned with the event (the person moved on): nothing to report.
      if (reply.reason === 'aborted') return result
      throw new Error(`ask-dont-tell: review of the final message failed (${reply.reason}); the turn ended unreviewed.`)
    }
    const verdict = Verdict.parse(reply.text)
    if (verdict === undefined) {
      throw new Error(`ask-dont-tell: reviewer reply was not the expected JSON; the turn ended unreviewed. Reply began: ${reply.text.slice(0, 120)}`)
    }
    if (verdict.kind === 'done' || running !== undefined) return result
    const draft = await $.prompt.read()
    if (draft.text.trim() !== '') return result

    // Not awaited: it resolves when the nudge's turn starts, after this hook returns.
    void $.prompt.submit({ text: Verdict.nudge(verdict.items) })
    return result
  })
}
