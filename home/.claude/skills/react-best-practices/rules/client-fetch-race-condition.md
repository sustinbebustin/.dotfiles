---
title: Handle Fetch Race Conditions with Cleanup
impact: MEDIUM-HIGH
impactDescription: prevents stale data from overwriting current results
tags: client, useEffect, fetch, race-condition, cleanup, data-fetching
---

## Handle Fetch Race Conditions with Cleanup

When fetching data in an effect, add a cleanup function that ignores stale responses. Without cleanup, rapid prop/state changes (e.g., typing in a search box) can cause older responses to arrive after newer ones, displaying wrong data.

**Incorrect (no cleanup, race condition):**

```tsx
function SearchResults({ query }: { query: string }) {
  const [results, setResults] = useState<Result[]>([])

  useEffect(() => {
    fetchResults(query).then(json => {
      setResults(json)
    })
  }, [query])

  return <ul>{results.map(r => <li key={r.id}>{r.title}</li>)}</ul>
}
```

Typing "hello" fires fetches for "h", "he", "hel", "hell", "hello". If "hell" responds after "hello", stale results overwrite current ones.

**Correct (ignore stale responses):**

```tsx
function SearchResults({ query }: { query: string }) {
  const [results, setResults] = useState<Result[]>([])

  useEffect(() => {
    let ignore = false
    fetchResults(query).then(json => {
      if (!ignore) {
        setResults(json)
      }
    })
    return () => {
      ignore = true
    }
  }, [query])

  return <ul>{results.map(r => <li key={r.id}>{r.title}</li>)}</ul>
}
```

When `query` changes, React runs the previous effect's cleanup, setting `ignore = true`. Only the most recent fetch updates state.

**Better: extract into a custom hook or use a library.** This pattern should live in a reusable hook or be replaced by a framework's built-in data fetching (SWR, React Query, Next.js server components):

```tsx
function useData<T>(url: string): T | null {
  const [data, setData] = useState<T | null>(null)
  useEffect(() => {
    let ignore = false
    fetch(url)
      .then(res => res.json())
      .then(json => {
        if (!ignore) setData(json)
      })
    return () => { ignore = true }
  }, [url])
  return data
}
```

References: [You Might Not Need an Effect - Fetching data](https://react.dev/learn/you-might-not-need-an-effect#fetching-data)
