/**
 * Waiting on You: a toast in every Claude Code session when another session is
 * blocked on the person (a permission prompt, an AskUserQuestion, an MCP form).
 *
 * Each session tracks its own waits from the hooks below and publishes them as
 * a beacon file, `<dir>/<sessionId>.json`, rewritten on every change and every
 * tick as a heartbeat. Each tick reads the other sessions' fresh beacons and
 * toasts each wait it has not seen. A toast is local to its session, and mods
 * have no channel between sessions, hence the files.
 *
 * Known gap: no event marks a permission prompt answered, so an approved call
 * counts as blocked until the tool finishes.
 */

import type { EngineInterface, Register, Timer } from 'claude-code'
import { Beacon, diffNewWaits, toastText, Wait } from './waits'

const TICK_MS = 2_000
const QUESTION_TOOL = 'AskUserQuestion'
/** Long enough to notice from another pane; the engine's 4 s default is easy to miss. */
const TOAST_MS = 15_000
/** A beacon this old is a crashed session's leftover and is deleted. */
const BEACON_PRUNE_MS = 60 * 60 * 1_000

type Running = {
  dir: string
  project: string
  waits: Wait[]
  /** Wait ids already toasted; `undefined` until the first tick seeds it, so a (re)load toasts nothing old. */
  seen: Set<string> | undefined
  timer: Timer
  isTicking: boolean
  hasWriteFailed: boolean
}

/** This session's state; `undefined` before `session.start` and after the session ends. */
let running: Running | undefined

const beaconDirOf = async ($: EngineInterface): Promise<string> => {
  const base = (await $.env.get('XDG_RUNTIME_DIR')) ?? (await $.env.get('TMPDIR')) ?? '/tmp'
  return `${base.replace(/\/+$/, '')}/claude-waiting-on-you`
}

const basename = (path: string): string => path.replace(/\/+$/, '').split('/').pop() || path

const publish = async ($: EngineInterface, state: Running): Promise<void> => {
  const sessionId = await $.session.id()
  const path = `${state.dir}/${sessionId}.json`
  const beacon: Beacon = { sessionId, project: state.project, waits: state.waits, seenAt: await $.clock.now() }
  try {
    await $.fs.write(path, Beacon.serialize(beacon))
    state.hasWriteFailed = false
  } catch (error) {
    // Said once per failing streak: the heartbeat would repeat it every tick.
    if (!state.hasWriteFailed) {
      $.ui.log(
        `could not write ${path} (${error instanceof Error ? error.message : String(error)}); other sessions will not see this one blocked until it can. Check that ${state.dir} is writable.`,
      )
    }
    state.hasWriteFailed = true
  }
}

const setWaits = async ($: EngineInterface, waits: Wait[]): Promise<void> => {
  if (running === undefined) return
  running.waits = waits
  await publish($, running)
}

const addWait = ($: EngineInterface, wait: Wait): Promise<void> =>
  setWaits($, [...(running?.waits ?? []).filter(held => held.key !== wait.key), wait])

const removeWait = async ($: EngineInterface, key: string): Promise<void> => {
  const waits = running?.waits ?? []
  if (waits.some(held => held.key === key)) await setWaits($, waits.filter(held => held.key !== key))
}

/**
 * Ends the permission wait for a finished tool call: the one whose input
 * matches, else the oldest for the same tool, since a hook above this one may
 * have rewritten the input the prompt was shown for.
 */
const endPermission = async ($: EngineInterface, tool: string, key: string): Promise<void> => {
  const waits = running?.waits ?? []
  const match =
    waits.find(held => held.key === key) ??
    waits.find(held => held.kind === 'permission' && held.key.startsWith(Wait.toolPrefix(tool)))
  if (match !== undefined) await setWaits($, waits.filter(held => held !== match))
}

const clearWaits = async ($: EngineInterface): Promise<void> => {
  if ((running?.waits.length ?? 0) > 0) await setWaits($, [])
}

const readBeacons = async ($: EngineInterface, state: Running, ownId: string, now: number): Promise<Beacon[]> => {
  let entries
  try {
    entries = await $.fs.list(state.dir)
  } catch {
    // The directory went away (a tmp cleaner): make it again and read next tick.
    await $.process.run(['mkdir', '-p', state.dir]).catch(() => undefined)
    return []
  }
  const names = entries.filter(entry => entry.kind === 'file' && entry.name.endsWith('.json') && entry.name !== `${ownId}.json`)
  const stale = names.filter(entry => now - entry.mtimeMs > BEACON_PRUNE_MS).map(entry => `${state.dir}/${entry.name}`)
  if (stale.length > 0) await $.process.run(['rm', '-f', ...stale]).catch(() => undefined)

  const read = await Promise.all(
    names
      .filter(entry => now - entry.mtimeMs <= BEACON_PRUNE_MS)
      .map(entry => $.fs.read(`${state.dir}/${entry.name}`).then(Beacon.parse, () => undefined)),
  )
  return read.filter((beacon): beacon is Beacon => beacon !== undefined && beacon.sessionId !== ownId && Beacon.isFresh(beacon, now))
}

const tick = async ($: EngineInterface, state: Running): Promise<void> => {
  if (state.isTicking) return
  state.isTicking = true
  try {
    await publish($, state)
    const ownId = await $.session.id()
    const beacons = await readBeacons($, state, ownId, await $.clock.now())
    const { fresh, current } = diffNewWaits(state.seen ?? new Set(), beacons)
    if (state.seen !== undefined) for (const sighting of fresh) $.ui.toast(toastText(sighting), { timeoutMs: TOAST_MS })
    state.seen = current
  } finally {
    state.isTicking = false
  }
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const result = await next(e)
    running?.timer.cancel()
    const dir = await beaconDirOf($)
    await $.process.run(['mkdir', '-p', dir]).catch(() => undefined)
    const state: Running = {
      dir,
      project: basename(e.cwd),
      waits: [],
      seen: undefined,
      timer: $.clock.every(TICK_MS, () => void tick($, state)),
      isTicking: false,
      hasWriteFailed: false,
    }
    running = state
    await tick($, state)
    return result
  })

  on('session.end', async ($, e, next) => {
    const state = running
    if (state !== undefined) {
      state.waits = []
      await $.process.run(['rm', '-f', `${state.dir}/${e.sessionId}.json`]).catch(() => undefined)
      // A /clear goes on under a new session id with no session.start: keep ticking.
      if (e.reason !== 'clear') {
        state.timer.cancel()
        running = undefined
      }
    }
    return next(e)
  })

  on('classic.PermissionRequest', async ($, e, next) => {
    const result = await next(e)
    // A settings hook beneath that decided means no prompt is shown. AskUserQuestion
    // asks permission too, but its question wait (tool.call) already covers the dialog.
    if (result.decision === undefined && e.tool_name !== QUESTION_TOOL) await addWait($, Wait.permission(e.tool_name, e.tool_input, await $.clock.now()))
    return result
  })

  on('tool.call', async ($, e, next) => {
    const { tool, tool_use_id, agentId: _agentId, requestMeta: _requestMeta, ...args } = e
    if (tool === QUESTION_TOOL) {
      const wait = Wait.question(tool_use_id, args, await $.clock.now())
      await addWait($, wait)
      try {
        return await next(e)
      } finally {
        await removeWait($, wait.key)
      }
    }
    try {
      return await next(e)
    } finally {
      await endPermission($, tool, Wait.toolKey(tool, args))
    }
  })

  on('classic.Elicitation', async ($, e, next) => {
    await addWait($, Wait.elicitation(e.mcp_server_name, e.elicitation_id, await $.clock.now()))
    return next(e)
  })

  on('classic.ElicitationResult', async ($, e, next) => {
    await removeWait($, Wait.elicitationKey(e.mcp_server_name, e.elicitation_id))
    return next(e)
  })

  // Backstops: a new prompt or a finished turn means nothing is waiting on the person.
  on('prompt.submit', async ($, e, next) => {
    await clearWaits($)
    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    await clearWaits($)
    return next(e)
  })
}
