---
title: Use useSyncExternalStore for Subscriptions
impact: MEDIUM
impactDescription: eliminates manual subscription boilerplate and tearing bugs
tags: client, useSyncExternalStore, useEffect, subscription, external-store
---

## Use useSyncExternalStore for Subscriptions

When subscribing to external data sources (browser APIs, third-party stores, global state outside React), use `useSyncExternalStore` instead of manual `addEventListener` + `useState` + `useEffect`. It handles subscription lifecycle, avoids tearing during concurrent renders, and supports server-side rendering.

**Incorrect (manual subscription in effect):**

```tsx
function useOnlineStatus() {
  const [isOnline, setIsOnline] = useState(true)

  useEffect(() => {
    function update() {
      setIsOnline(navigator.onLine)
    }
    update()
    window.addEventListener('online', update)
    window.addEventListener('offline', update)
    return () => {
      window.removeEventListener('online', update)
      window.removeEventListener('offline', update)
    }
  }, [])

  return isOnline
}
```

**Correct (useSyncExternalStore):**

```tsx
function subscribe(callback: () => void) {
  window.addEventListener('online', callback)
  window.addEventListener('offline', callback)
  return () => {
    window.removeEventListener('online', callback)
    window.removeEventListener('offline', callback)
  }
}

function useOnlineStatus() {
  return useSyncExternalStore(
    subscribe,
    () => navigator.onLine,
    () => true
  )
}
```

The three arguments: (1) subscribe function that returns an unsubscribe function, (2) client snapshot getter, (3) server snapshot getter. React will not resubscribe as long as the same `subscribe` function reference is passed -- hoist it outside the component.

**When to use:** Any data that lives outside React's state system -- browser APIs (`navigator`, `matchMedia`, `localStorage`), WebSocket connections, third-party state managers without React bindings, shared workers.

References: [You Might Not Need an Effect - Subscribing to an external store](https://react.dev/learn/you-might-not-need-an-effect#subscribing-to-an-external-store), [useSyncExternalStore API](https://react.dev/reference/react/useSyncExternalStore)
