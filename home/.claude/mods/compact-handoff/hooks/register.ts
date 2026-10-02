import type { ModelForkResult, Register, SessionMessage } from 'claude-code'

const HANDOFF_PROMPT = `The conversation is about to be compacted: it will be replaced with a summary, and you keep working in this same session afterwards. Early instructions, tool output, and reasoning you have not written down will be gone.

Write the handoff that carries this work across. The reader is you, right after compaction. Record what you would be stuck without:
- the user's decisions and constraints, as they stated them
- what you have established and ruled out, with its evidence (path:line, command output)
- the exact state of in-flight work: what is done, what is half-done, which files changed
- the next concrete action

Reference specs, plans, issues, commits, and diffs by path or URL; their content stays where it is. Redact secrets and credentials. Keep it under 5,000 tokens.

Reply with the handoff document alone, in Markdown. Your tools are unavailable for this reply.`

const handoffPrompt = (instructions: string | undefined): string =>
  instructions === undefined || instructions.trim() === ''
    ? HANDOFF_PROMPT
    : `${HANDOFF_PROMPT}\n\nAfter compacting, the session will do this: ${instructions.trim()}\nWeight the handoff toward what that work needs.`

type ForkFailure = Extract<ModelForkResult, { isAnswered: false }>

const describeFailure = (failure: ForkFailure): string => {
  switch (failure.reason) {
    case 'api-error':
      return `the API returned ${failure.error} (status ${failure.status ?? 'none'})`
    case 'empty-reply':
      return 'the model replied with no text'
    case 'aborted':
      return 'the request was interrupted'
    case 'nothing-to-fork':
      return 'the conversation has no response yet'
  }
}

const handoffMessage = (text: string, path: string | undefined): SessionMessage => ({
  role: 'user',
  text: `${path === undefined ? 'Handoff you wrote just before this compaction' : `Handoff you wrote just before this compaction (also saved at ${path})`}. It is your own record of the work; read it with the summary above.\n\n${text}`,
  toolUses: [],
})

export const register: Register = on => {
  on('session.compact', async ($, e, next) => {
    // A subagent's own compaction, and a precompute whose result is kept for later, carry no handoff.
    if (e.agentId !== undefined || e.trigger === 'precompute') return next(e)

    const fork = await $.model.fork({ prompt: handoffPrompt(e.instructions) })
    if (!fork.isAnswered) {
      const why = describeFailure(fork)
      // A manual /compact can wait: leave the conversation whole so no work is lost.
      if (e.trigger === 'manual') {
        return {
          skip: `compact-handoff: the handoff was not written because ${why}. The conversation is unchanged. Run /compact again, or disable compact-handoff in /plugin to compact without a handoff.`,
        }
      }
      $.ui.log(`the handoff was not written because ${why}; compacting without one.`)
      return next(e)
    }

    const sessionId = await $.session.id()
    const tmp = (await $.env.get('TMPDIR')) ?? '/tmp'
    const target = `${tmp.replace(/\/$/, '')}/claude-handoff-${sessionId}.md`
    const saved = await $.fs.write(target, fork.text).then(
      () => target,
      () => undefined,
    )
    if (saved === undefined) $.ui.log(`could not save the handoff to ${target}; it is still carried into the conversation.`)

    const compacted = await next(e)
    if (compacted.skip !== undefined) return compacted

    $.ui.log(saved === undefined ? 'handoff carried across compaction.' : `handoff carried across compaction; saved at ${saved}`)
    return { ...compacted, messages: [...compacted.messages, handoffMessage(fork.text, saved)] }
  })
}
