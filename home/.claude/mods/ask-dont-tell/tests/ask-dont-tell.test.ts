import type { On } from 'claude-code'
import { type Engine, expect, test } from 'claude-code/testing'
import { Verdict } from '../hooks/verdict'

const USAGE = { input_tokens: 1, output_tokens: 1, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 }

/** The engine beneath the plugin: a reviewer that answers `answer`, a prompt box holding `draft`. */
const world = (on: On, answer: string, draft = '') => {
  const reviewed: string[] = []
  const submitted: string[] = []
  on('model.complete', ($, e) => {
    reviewed.push(e.prompt)
    return { value: { isAnswered: true as const, text: answer, usage: USAGE } }
  })
  on('prompt.read', () => ({ value: { text: draft, cursor: draft.length } }))
  on('prompt.submit', ($, e) => {
    submitted.push(e.text)
    return { text: e.text }
  })
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('turn.start', ($, e) => ({ turnId: e.turnId }))
  on('turn.complete', () => ({ text: '' }))
  return { reviewed, submitted }
}

const startSession = ($: Engine, isInteractive = true) =>
  $.session.start({ cwd: '/home/austin/projects/api', surface: 'terminal', isInteractive })

/** One main-loop turn: the person's `prompt`, then Claude's final `answer`. */
const turn = async ($: Engine, prompt: string, answer: string, end: { reason?: 'answer' | 'aborted'; agentId?: string } = {}) => {
  const turnId = `t-${prompt.length}-${answer.length}`
  await $.turn.start({ turnId, text: prompt, ...(end.agentId === undefined ? {} : { agentId: end.agentId }) })
  return $.turn.complete({
    turnId,
    answer,
    durationMs: 1,
    isAborted: end.reason === 'aborted',
    reason: end.reason ?? 'answer',
    ...(end.agentId === undefined ? {} : { agentId: end.agentId }),
  })
}

const OPEN = '{"items": [{"what": "Still to do: push the branch", "kind": "todo"}]}'
const DONE = '{"items": []}'

// --- verdict.ts

test('Verdict.parse reads the JSON, tolerates a fence, and rejects other shapes', () => {
  expect(Verdict.parse(DONE)).toEqual({ kind: 'done' })
  expect(Verdict.parse(OPEN)).toEqual({ kind: 'open', items: ['Still to do: push the branch'] })
  expect(Verdict.parse('```json\n{"items": [{"what": "a", "kind": "ask"}, {"what": 3, "kind": "ask"}]}\n```')).toEqual({ kind: 'open', items: ['a'] })
  expect(Verdict.parse('{"items": "none"}')).toBeUndefined()
  expect(Verdict.parse('Sure! {"items": []}')).toBeUndefined()
})

test('Verdict.parse nudges for asks, offers and todos, never for running, later or fyi items', () => {
  const items = (kind: string) => `{"items": [{"what": "x", "kind": "${kind}"}]}`
  for (const kind of ['ask', 'offer', 'todo']) expect(Verdict.parse(items(kind))).toEqual({ kind: 'open', items: ['x'] })
  for (const kind of ['running', 'later', 'fyi', 'unknown']) expect(Verdict.parse(items(kind))).toEqual({ kind: 'done' })
  const mixed = '{"items": [{"what": "test run, an hour left", "kind": "running"}, {"what": "push it?", "kind": "offer"}]}'
  expect(Verdict.parse(mixed)).toEqual({ kind: 'open', items: ['push it?'] })
})

test('Verdict.nudge names each item and the tool, and is recognised as a nudge', () => {
  const nudge = Verdict.nudge(['Pick Postgres or SQLite'])
  expect(nudge).toContain('- Pick Postgres or SQLite')
  expect(nudge).toContain('AskUserQuestion')
  expect(Verdict.isNudge(nudge)).toBe(true)
  expect(Verdict.isNudge("what's next?")).toBe(false)
})

// --- the turn.complete hook

test('a reply left open submits a nudge with the items', async ($, on) => {
  const w = world(on, OPEN)
  await startSession($)
  await turn($, 'Fix the deploy', 'Deployed.\n\nStill to do:\n- push the branch')

  expect(w.submitted.length).toBe(1)
  expect(w.submitted[0]).toContain('- Still to do: push the branch')
  expect(w.reviewed[0]).toContain('<user_prompt>\nFix the deploy\n</user_prompt>')
  expect(w.reviewed[0]).toContain('push the branch')
})

test('a reply the reviewer clears submits nothing', async ($, on) => {
  const w = world(on, DONE)
  await startSession($)
  await turn($, 'What does ui/ hold?', 'The built-in web UI.')
  expect(w.reviewed.length).toBe(1)
  expect(w.submitted).toEqual([])
})

test('the turn a nudge started is not reviewed', async ($, on) => {
  const w = world(on, OPEN)
  await startSession($)
  await turn($, Verdict.nudge(['push the branch']), 'Still to do: push the branch')
  expect(w.reviewed).toEqual([])
  expect(w.submitted).toEqual([])
})

test('a person already typing a reply gets no nudge', async ($, on) => {
  const w = world(on, OPEN, 'yes, push it')
  await startSession($)
  await turn($, 'Fix the deploy', 'Want me to push?')
  expect(w.reviewed.length).toBe(1)
  expect(w.submitted).toEqual([])
})

test('interrupted and subagent turns are never reviewed', async ($, on) => {
  const w = world(on, OPEN)
  await startSession($)
  await turn($, 'Fix it', 'Want me to push?', { reason: 'aborted' })
  await turn($, 'Search', 'Want me to push?', { agentId: 'a1' })
  expect(w.reviewed).toEqual([])
})

test('a non-interactive session is never reviewed', async ($, on) => {
  const w = world(on, OPEN)
  await startSession($, false)
  await turn($, 'Fix it', 'Want me to push?')
  expect(w.reviewed).toEqual([])
})
