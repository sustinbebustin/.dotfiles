/**
 * Ask, Don't Tell: when a turn ends on a reply that leaves a question, a
 * decision or an offer for the person in prose, block the stop and tell Claude
 * to ask it with AskUserQuestion.
 *
 * A regex screen (Reply.mayAwaitUser) keeps most turns free; only a reply it
 * flags costs a small-model call. One nudge per turn: a stop that a Stop hook
 * already continued (`stop_hook_active`) is let through, so a reply the model
 * keeps misjudging cannot loop.
 */

import type { Register } from 'claude-code'
import { Reply, Verdict } from './verdict'

const MODEL = 'haiku'
const REVIEW_TIMEOUT_MS = 15_000

export const register: Register = (on) => {
  /** AskUserQuestion needs a person at the prompt; `claude -p` and SDK hosts have none. */
  let isInteractive = false

  on('session.start', async ($, e, next) => {
    isInteractive = e.isInteractive
    return next(e)
  })

  on('classic.Stop', async ($, e, next) => {
    const result = await next(e)
    if (result.block !== undefined || !isInteractive || e.stop_hook_active) return result
    const text = e.last_assistant_message
    if (text === undefined || !Reply.mayAwaitUser(text)) return result

    const reply = await $.model.complete({
      model: MODEL,
      system: Verdict.SYSTEM,
      prompt: `<final_message>\n${text}\n</final_message>`,
      maxTokens: 400,
      effort: 'low',
      timeoutMs: REVIEW_TIMEOUT_MS,
    })
    if (!reply.isAnswered) {
      throw new Error(`ask-dont-tell: review of the final message failed (${reply.reason}); the turn ended unreviewed.`)
    }
    const verdict = Verdict.parse(reply.text)
    if (verdict === undefined) {
      throw new Error(`ask-dont-tell: reviewer reply was not the expected JSON; the turn ended unreviewed. Reply began: ${reply.text.slice(0, 120)}`)
    }
    return verdict.kind === 'waiting' ? { ...result, block: Verdict.blockReason(verdict.items) } : result
  })
}
