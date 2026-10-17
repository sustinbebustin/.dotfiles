/**
 * What a session is blocked on, and the beacon file each session publishes so
 * every other session can see it. Pure: no `$`.
 */

export type WaitKind = 'permission' | 'question' | 'elicitation'

/** One thing a session waits on the person for. `key` matches the wait's start to its end. */
export type Wait = {
  kind: WaitKind
  key: string
  label: string
  since: number
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

const isWaitKind = (value: unknown): value is WaitKind =>
  value === 'permission' || value === 'question' || value === 'elicitation'

const isWait = (value: unknown): value is Wait =>
  isRecord(value) &&
  isWaitKind(value.kind) &&
  typeof value.key === 'string' &&
  typeof value.label === 'string' &&
  typeof value.since === 'number'

/** JSON with object keys sorted, so equal inputs give equal keys whatever their key order. */
const canonicalJson = (value: unknown): string =>
  JSON.stringify(value, (_key, inner: unknown) =>
    isRecord(inner) ? Object.fromEntries(Object.entries(inner).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))) : inner,
  ) ?? 'undefined'

const LABEL_DETAIL_MAX = 60

const clip = (text: string): string => {
  const line = text.replace(/\s+/g, ' ').trim()
  return line.length > LABEL_DETAIL_MAX ? `${line.slice(0, LABEL_DETAIL_MAX - 1)}…` : line
}

/** The argument that says most about a tool call, for the toast. */
const toolDetail = (input: unknown): string | undefined => {
  if (!isRecord(input)) return undefined
  for (const field of ['command', 'file_path', 'notebook_path', 'url', 'pattern', 'query']) {
    const value = input[field]
    if (typeof value === 'string' && value.trim() !== '') return clip(value)
  }
  return undefined
}

const firstQuestion = (input: unknown): string | undefined => {
  if (!isRecord(input) || !Array.isArray(input.questions)) return undefined
  const [first] = input.questions
  return isRecord(first) && typeof first.question === 'string' ? clip(first.question) : undefined
}

export const Wait = {
  /** What every `toolKey` of `tool` starts with. */
  toolPrefix: (tool: string): string => `tool:${tool}:`,

  /** The key a permission prompt and the tool call it gates share. */
  toolKey: (tool: string, input: unknown): string => `${Wait.toolPrefix(tool)}${canonicalJson(input)}`,

  permission: (tool: string, input: unknown, since: number): Wait => {
    const detail = toolDetail(input)
    return {
      kind: 'permission',
      key: Wait.toolKey(tool, input),
      label: detail === undefined ? `approve ${tool}` : `approve ${tool}: ${detail}`,
      since,
    }
  },

  question: (toolUseId: string, input: unknown, since: number): Wait => {
    const question = firstQuestion(input)
    return {
      kind: 'question',
      key: `question:${toolUseId}`,
      label: question === undefined ? 'answer a question' : `answer: ${question}`,
      since,
    }
  },

  /** The result carries the server and id but not the message, so those alone match the two. */
  elicitationKey: (server: string, id: string | undefined): string => `elicitation:${server}:${id ?? ''}`,

  elicitation: (server: string, id: string | undefined, since: number): Wait => ({
    kind: 'elicitation',
    key: Wait.elicitationKey(server, id),
    label: `MCP ${server} input`,
    since,
  }),
} as const

/** One session's published state: the file `<dir>/<sessionId>.json`. `seenAt` is its heartbeat. */
export type Beacon = {
  sessionId: string
  project: string
  waits: readonly Wait[]
  seenAt: number
}

/** A beacon whose heartbeat is older than this belongs to a session that is gone. */
export const BEACON_MAX_AGE_MS = 10_000

export const Beacon = {
  /** `undefined` for anything that is not a whole beacon: another process may be mid-write. */
  parse: (text: string): Beacon | undefined => {
    let value: unknown
    try {
      value = JSON.parse(text)
    } catch {
      return undefined
    }
    if (
      !isRecord(value) ||
      typeof value.sessionId !== 'string' ||
      typeof value.project !== 'string' ||
      typeof value.seenAt !== 'number' ||
      !Array.isArray(value.waits) ||
      !value.waits.every(isWait)
    ) {
      return undefined
    }
    return { sessionId: value.sessionId, project: value.project, waits: value.waits, seenAt: value.seenAt }
  },

  serialize: (beacon: Beacon): string => JSON.stringify(beacon),

  isFresh: (beacon: Beacon, now: number): boolean => now - beacon.seenAt <= BEACON_MAX_AGE_MS,
} as const

/** Identifies one wait across ticks: a wait that ends and starts again is a new one. */
const waitId = (beacon: Beacon, wait: Wait): string => `${beacon.sessionId}|${wait.key}|${wait.since}`

export type Sighting = { beacon: Beacon; wait: Wait }

/**
 * The waits in `beacons` not in `seen`, and the ids of every wait in `beacons`
 * to carry into the next tick (ended waits drop out, so `seen` stays small).
 */
export const diffNewWaits = (
  seen: ReadonlySet<string>,
  beacons: readonly Beacon[],
): { fresh: Sighting[]; current: Set<string> } => {
  const fresh: Sighting[] = []
  const current = new Set<string>()
  for (const beacon of beacons) {
    for (const wait of beacon.waits) {
      const id = waitId(beacon, wait)
      current.add(id)
      if (!seen.has(id)) fresh.push({ beacon, wait })
    }
  }
  return { fresh, current }
}

export const toastText = ({ beacon, wait }: Sighting): string => `Claude blocked in ${beacon.project}: ${wait.label}`
