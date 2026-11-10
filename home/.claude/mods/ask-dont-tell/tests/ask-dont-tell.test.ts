import type { On } from 'claude-code'
import { type Engine, expect, test } from 'claude-code/testing'
import { Reply, Verdict } from '../hooks/verdict'

const USAGE = { input_tokens: 1, output_tokens: 1, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 }

/** The engine beneath the plugin: a reviewer that answers `answer`, and no settings Stop hook. */
const world = (on: On, answer: string) => {
  const prompts: string[] = []
  on('model.complete', ($, e) => {
    prompts.push(e.prompt)
    return { value: { isAnswered: true as const, text: answer, usage: USAGE } }
  })
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('classic.Stop', () => ({}))
  return { prompts }
}

const startSession = ($: Engine, isInteractive = true) =>
  $.session.start({ cwd: '/home/austin/projects/api', surface: 'terminal', isInteractive })

const stop = ($: Engine, last_assistant_message: string, stop_hook_active = false) =>
  $.classic.Stop({ session_id: 's', transcript_path: '/tmp/t.jsonl', cwd: '/home/austin/projects/api', stop_hook_active, last_assistant_message })

const WAITING = '{"waiting": true, "items": ["Want me to add tests?"]}'

// --- verdict.ts

test('Reply.mayAwaitUser flags questions and cues in prose only', () => {
  expect(Reply.mayAwaitUser('Done. Want me to add tests?')).toBe(true)
  expect(Reply.mayAwaitUser('Next steps:\n- wire the API')).toBe(true)
  expect(Reply.mayAwaitUser('Let me know which you prefer.')).toBe(true)
  expect(Reply.mayAwaitUser('Fixed the null check in `parse`. Tests pass.')).toBe(false)
  expect(Reply.mayAwaitUser('Fetches `https://x.dev/a?q=1` now.')).toBe(false)
  expect(Reply.mayAwaitUser('```ts\nconst a = b ? c : d // should I?\n```\nRefactored.')).toBe(false)
})

test('Verdict.parse reads the JSON, tolerates a fence, and rejects other shapes', () => {
  expect(Verdict.parse('{"waiting": false}')).toEqual({ kind: 'done' })
  expect(Verdict.parse(WAITING)).toEqual({ kind: 'waiting', items: ['Want me to add tests?'] })
  expect(Verdict.parse('```json\n{"waiting": true, "items": ["a", 3]}\n```')).toEqual({ kind: 'waiting', items: ['a'] })
  expect(Verdict.parse('{"waiting": "yes"}')).toBeUndefined()
  expect(Verdict.parse('Sure! {"waiting": true}')).toBeUndefined()
})

test('Verdict.blockReason names each item and the tool', () => {
  const reason = Verdict.blockReason(['Pick Postgres or SQLite'])
  expect(reason).toContain('- Pick Postgres or SQLite')
  expect(reason).toContain('AskUserQuestion')
})

// --- the Stop hook

test('a reply waiting on the person blocks the stop with the items', async ($, on) => {
  const w = world(on, WAITING)
  await startSession($)
  const result = await stop($, 'Done. Want me to add tests?')

  expect(result.block).toContain('- Want me to add tests?')
  expect(w.prompts[0]).toContain('Want me to add tests?')
})

test('a reply the reviewer clears ends the turn', async ($, on) => {
  world(on, '{"waiting": false}')
  await startSession($)
  expect((await stop($, 'Tests pass? Yes: 42 of 42.')).block).toBeUndefined()
})

test('a reply the screen passes costs no model call', async ($, on) => {
  const w = world(on, WAITING)
  await startSession($)
  expect((await stop($, 'Fixed the off-by-one. Tests pass.')).block).toBeUndefined()
  expect(w.prompts).toEqual([])
})

test('a stop already continued by a Stop hook is let through', async ($, on) => {
  const w = world(on, WAITING)
  await startSession($)
  expect((await stop($, 'Want me to add tests?', true)).block).toBeUndefined()
  expect(w.prompts).toEqual([])
})

test('a non-interactive session is never blocked', async ($, on) => {
  const w = world(on, WAITING)
  await startSession($, false)
  expect((await stop($, 'Want me to add tests?')).block).toBeUndefined()
  expect(w.prompts).toEqual([])
})
