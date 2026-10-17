import type { On } from 'claude-code'
import { type Engine, expect, mock, test } from 'claude-code/testing'
import { Beacon, diffNewWaits, toastText, Wait } from '../hooks/waits'

const DIR = '/run/user/1000/claude-waiting-on-you'
const OWN = 'own-session'

/** An in-memory file system, session id and environment under the plugin. */
const world = (on: On, start = 1_000_000) => {
  const files = new Map<string, { text: string; mtimeMs: number }>()
  const toasts: string[] = []
  const toastTimeouts: (number | undefined)[] = []
  const clock = mock.clock(on, { now: start })
  mock.env(on, { XDG_RUNTIME_DIR: '/run/user/1000/' })
  on('session.id', () => ({ value: OWN }))
  on('fs.write', ($, e) => {
    files.set(e.path, { text: e.text, mtimeMs: clock.now() })
    return { value: undefined }
  })
  on('fs.read', ($, e) => {
    const file = files.get(e.path)
    if (file === undefined) throw new Error(`ENOENT: ${e.path}`)
    return { value: file.text }
  })
  on('fs.list', ($, e) => ({
    value: [...files.entries()]
      .filter(([path]) => path.startsWith(`${e.path}/`))
      .map(([path, file]) => ({ name: path.slice(e.path.length + 1), kind: 'file' as const, size: file.text.length, mtimeMs: file.mtimeMs, isLink: false })),
  }))
  on('process.run', ($, e) => {
    if (e.argv[0] === 'rm') for (const path of e.argv.slice(2)) files.delete(path)
    return { value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  on('ui.toast', ($, e) => {
    toasts.push(e.text)
    toastTimeouts.push(e.timeoutMs)
    return { value: undefined }
  })
  on('ui.log', () => ({ value: undefined }))
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('session.end', ($, e) => ({ sessionId: e.sessionId }))
  on('prompt.submit', ($, e) => ({ text: e.text }))
  // No settings hook decides: the prompt is shown.
  on('classic.PermissionRequest', () => ({}))
  on('classic.Elicitation', () => ({}))
  on('classic.ElicitationResult', () => ({}))

  const beaconOf = (sessionId: string): Beacon | undefined => {
    const file = files.get(`${DIR}/${sessionId}.json`)
    return file === undefined ? undefined : Beacon.parse(file.text)
  }
  const putBeacon = (beacon: Beacon): void => {
    files.set(`${DIR}/${beacon.sessionId}.json`, { text: Beacon.serialize(beacon), mtimeMs: clock.now() })
  }
  return { files, toasts, toastTimeouts, clock, beaconOf, putBeacon }
}

const startSession = ($: Engine) =>
  $.session.start({ cwd: '/home/austin/projects/api', surface: 'terminal', isInteractive: true })

const permissionInput = (tool_name: string, tool_input: Record<string, string>) => ({
  session_id: OWN,
  transcript_path: '/tmp/t.jsonl',
  cwd: '/home/austin/projects/api',
  tool_name,
  tool_input,
})

// --- waits.ts

test('Beacon.parse returns undefined for a half-written or wrong-shaped file', () => {
  const beacon: Beacon = { sessionId: 's', project: 'p', waits: [Wait.permission('Bash', { command: 'ls' }, 1)], seenAt: 2 }
  const text = Beacon.serialize(beacon)

  expect(Beacon.parse(text)).toEqual(beacon)
  expect(Beacon.parse(text.slice(0, text.length - 5))).toBeUndefined()
  expect(Beacon.parse('{"sessionId":"s","project":"p","waits":[{"kind":"nap"}],"seenAt":2}')).toBeUndefined()
  expect(Beacon.parse('[]')).toBeUndefined()
})

test('Wait.toolKey ignores key order, so the prompt and the call it gates share a key', () => {
  expect(Wait.toolKey('Edit', { file_path: 'a', old_string: 'x' })).toBe(Wait.toolKey('Edit', { old_string: 'x', file_path: 'a' }))
  expect(Wait.toolKey('Edit', { file_path: 'a' })).not.toBe(Wait.toolKey('Edit', { file_path: 'b' }))
})

test('labels name what the person is asked', () => {
  expect(Wait.permission('Bash', { command: 'git   push\norigin' }, 0).label).toBe('approve Bash: git push origin')
  expect(Wait.permission('WebSearch', {}, 0).label).toBe('approve WebSearch')
  expect(Wait.question('t1', { questions: [{ question: 'Which DB?' }] }, 0).label).toBe('answer: Which DB?')
  expect(Wait.elicitation('github', 'e1', 0).label).toBe('MCP github input')
})

test('diffNewWaits reports unseen waits and drops ended ones from what it carries', () => {
  const wait = Wait.question('t1', {}, 5)
  const beacon: Beacon = { sessionId: 'b', project: 'web', waits: [wait], seenAt: 5 }

  const first = diffNewWaits(new Set(), [beacon])
  expect(first.fresh.map(toastText)).toEqual(['Claude blocked in web: answer a question'])

  const second = diffNewWaits(first.current, [beacon])
  expect(second.fresh).toEqual([])

  const ended = diffNewWaits(second.current, [{ ...beacon, waits: [] }])
  expect(ended.current.size).toBe(0)
})

// --- this session's waits

test('a permission prompt is published until the call it gates resolves', async ($, on) => {
  const w = world(on)
  let duringCall: Beacon | undefined
  on('tool.call', () => {
    duringCall = w.beaconOf(OWN)
    return { deny: 'denied in the dialog' }
  })
  await startSession($)

  await $.classic.PermissionRequest(permissionInput('Bash', { command: 'git push' }))
  expect(w.beaconOf(OWN)?.waits.map(wait => wait.label)).toEqual(['approve Bash: git push'])

  await $.tool.call({ tool: 'Bash', command: 'git push' })
  expect(duringCall?.waits).toHaveLength(1)
  expect(w.beaconOf(OWN)?.waits).toEqual([])
})

test('an AskUserQuestion is published while it is open and cleared once answered', async ($, on) => {
  const w = world(on)
  let duringCall: Beacon | undefined
  on('tool.call', () => {
    duringCall = w.beaconOf(OWN)
    return { deny: 'stand-in for the answer' }
  })
  await startSession($)

  await $.tool.call({ tool: 'AskUserQuestion', questions: [{ question: 'Which DB?', header: 'DB', options: [], multiSelect: false }] })

  expect(duringCall?.waits.map(wait => wait.label)).toEqual(['answer: Which DB?'])
  expect(w.beaconOf(OWN)?.waits).toEqual([])
})

test("AskUserQuestion's own permission request adds no second wait", async ($, on) => {
  const w = world(on)
  let duringCall: Beacon | undefined
  on('tool.call', async () => {
    await $.classic.PermissionRequest(permissionInput('AskUserQuestion', { questions: 'Which DB?' }))
    duringCall = w.beaconOf(OWN)
    return { deny: 'stand-in for the answer' }
  })
  await startSession($)

  await $.tool.call({ tool: 'AskUserQuestion', questions: [{ question: 'Which DB?', header: 'DB', options: [], multiSelect: false }] })

  expect(duringCall?.waits.map(wait => wait.label)).toEqual(['answer: Which DB?'])
})

test('an MCP elicitation is published until its result', async ($, on) => {
  const w = world(on)
  await startSession($)
  const base = { session_id: OWN, transcript_path: '/tmp/t.jsonl', cwd: '/x', mcp_server_name: 'github', elicitation_id: 'e1' }

  await $.classic.Elicitation({ ...base, message: 'Pick a repo' })
  expect(w.beaconOf(OWN)?.waits.map(wait => wait.label)).toEqual(['MCP github input'])

  await $.classic.ElicitationResult({ ...base, action: 'accept' })
  expect(w.beaconOf(OWN)?.waits).toEqual([])
})

test('a new prompt clears any wait left behind', async ($, on) => {
  const w = world(on)
  await startSession($)
  await $.classic.PermissionRequest(permissionInput('Bash', { command: 'make' }))

  await $.prompt.submit({ text: 'never mind', wait: false, origin: { kind: 'composer' } })

  expect(w.beaconOf(OWN)?.waits).toEqual([])
})

test('the session ending removes its beacon', async ($, on) => {
  const w = world(on)
  await startSession($)
  expect(w.beaconOf(OWN)).toBeDefined()

  await $.session.end({ reason: 'other', sessionId: OWN, resume: { id: OWN } })

  expect(w.beaconOf(OWN)).toBeUndefined()
})

// --- other sessions' waits

test("another session's new wait toasts once", async ($, on) => {
  const w = world(on)
  await startSession($)

  w.putBeacon({ sessionId: 'other', project: 'web', waits: [Wait.permission('Bash', { command: 'rm -rf dist' }, w.clock.now())], seenAt: w.clock.now() })
  await w.clock.advance(2_000)
  w.putBeacon({ ...(w.beaconOf('other') ?? { sessionId: 'other', project: 'web', waits: [] }), seenAt: w.clock.now() })
  await w.clock.advance(2_000)

  expect(w.toasts).toEqual(['Claude blocked in web: approve Bash: rm -rf dist'])
  expect(w.toastTimeouts).toEqual([15_000])
})

test("this session's own waits never toast here", async ($, on) => {
  const w = world(on)
  await startSession($)

  await $.classic.PermissionRequest(permissionInput('Bash', { command: 'make' }))
  await w.clock.advance(2_000)

  expect(w.toasts).toEqual([])
})

test('a stale beacon from a crashed session is ignored', async ($, on) => {
  const w = world(on)
  await startSession($)

  w.putBeacon({ sessionId: 'gone', project: 'web', waits: [Wait.question('t1', {}, 0)], seenAt: w.clock.now() - 60_000 })
  await w.clock.advance(2_000)

  expect(w.toasts).toEqual([])
})

test('waits already open when the session starts do not toast', async ($, on) => {
  const w = world(on)
  w.putBeacon({ sessionId: 'other', project: 'web', waits: [Wait.question('t1', {}, w.clock.now())], seenAt: w.clock.now() })

  await startSession($)
  w.putBeacon({ ...(w.beaconOf('other') ?? { sessionId: 'other', project: 'web', waits: [] }), seenAt: w.clock.now() })
  await w.clock.advance(2_000)

  expect(w.toasts).toEqual([])
})
