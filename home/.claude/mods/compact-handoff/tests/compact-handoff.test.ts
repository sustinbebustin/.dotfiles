import type { SessionMessage } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

const usage = { input_tokens: 10, output_tokens: 5, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 }
const summary: SessionMessage = { role: 'user', text: 'Summary of the conversation.', toolUses: [] }
const transcript: SessionMessage[] = [{ role: 'user', text: 'Fix the parser.', toolUses: [] }]

test('a manual compaction carries the handoff in after the summary and saves it', async ($, on) => {
  const prompts: string[] = []
  const writes: string[] = []
  on('model.fork', ($, e) => {
    prompts.push(e.prompt)
    return { value: { isAnswered: true, text: '# Handoff\nNext: run the tests.', usage } }
  })
  on('session.id', () => ({ value: 'abc' }))
  mock.env(on, { TMPDIR: '/tmp/x/' })
  on('fs.write', ($, e) => {
    writes.push(e.path)
    return { value: undefined }
  })
  on('ui.log', () => ({ value: undefined }))
  on('session.compact', () => ({ messages: [summary] }))

  const result = await $.session.compact({ trigger: 'manual', instructions: 'ship the release', messages: transcript })

  expect(prompts[0]).toContain('After compacting, the session will do this: ship the release')
  expect(writes).toEqual(['/tmp/x/claude-handoff-abc.md'])
  expect(result.skip).toBeUndefined()
  expect(result.messages?.[0]).toEqual(summary)
  expect(result.messages?.[1]?.role).toBe('user')
  expect(result.messages?.[1]?.text).toContain('saved at /tmp/x/claude-handoff-abc.md')
  expect(result.messages?.[1]?.text).toContain('Next: run the tests.')
})

test('a manual compaction is skipped, conversation intact, when the handoff fails', async ($, on) => {
  let compacted = false
  on('model.fork', () => ({ value: { isAnswered: false, reason: 'aborted', usage } }))
  on('session.compact', () => {
    compacted = true
    return { messages: [summary] }
  })

  const result = await $.session.compact({ trigger: 'manual', messages: transcript })

  expect(compacted).toBe(false)
  expect(result.skip).toContain('the request was interrupted')
})

test('a precompute passes through without a handoff', async ($, on) => {
  let forked = false
  on('model.fork', () => {
    forked = true
    return { value: { isAnswered: true, text: 'x', usage } }
  })
  on('session.compact', () => ({ messages: [summary] }))

  const result = await $.session.compact({ trigger: 'precompute', messages: transcript })

  expect(forked).toBe(false)
  expect(result.messages).toEqual([summary])
})
