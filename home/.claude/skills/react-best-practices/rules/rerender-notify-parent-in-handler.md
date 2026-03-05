---
title: Notify Parent in Event Handler
impact: MEDIUM
impactDescription: avoids extra render pass from effect-driven parent updates
tags: rerender, useEffect, onChange, parent, state-sync, lifting-state
---

## Notify Parent in Event Handler

When a child component needs to notify its parent of state changes, call the parent callback in the same event handler that updates local state. Do not use useEffect to watch state and call `onChange` -- this causes an extra render pass (child renders with new state, then effect fires, then parent re-renders).

**Incorrect (effect notifies parent too late):**

```tsx
function Toggle({ onChange }: { onChange: (isOn: boolean) => void }) {
  const [isOn, setIsOn] = useState(false)

  useEffect(() => {
    onChange(isOn)
  }, [isOn, onChange])

  function handleClick() {
    setIsOn(!isOn)
  }

  return <button onClick={handleClick}>{isOn ? 'On' : 'Off'}</button>
}
```

**Correct (notify in handler, React batches both updates):**

```tsx
function Toggle({ onChange }: { onChange: (isOn: boolean) => void }) {
  const [isOn, setIsOn] = useState(false)

  function updateToggle(nextIsOn: boolean) {
    setIsOn(nextIsOn)
    onChange(nextIsOn)
  }

  function handleClick() {
    updateToggle(!isOn)
  }

  return <button onClick={handleClick}>{isOn ? 'On' : 'Off'}</button>
}
```

React batches state updates from the same event handler, so both the child and parent re-render in a single pass.

**Also consider: fully controlled component.** If the parent already tracks this state, remove local state entirely and let the parent own it:

```tsx
function Toggle({ isOn, onChange }: { isOn: boolean; onChange: (isOn: boolean) => void }) {
  return <button onClick={() => onChange(!isOn)}>{isOn ? 'On' : 'Off'}</button>
}
```

References: [You Might Not Need an Effect - Notifying parent components about state changes](https://react.dev/learn/you-might-not-need-an-effect#notifying-parent-components-about-state-changes)
