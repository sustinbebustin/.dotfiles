---
title: Reset State with Key Prop
impact: MEDIUM
impactDescription: avoids stale state and unnecessary effect-driven resets
tags: rerender, key, state-reset, useEffect, props
---

## Reset State with Key Prop

When a component should reset all its state in response to a prop change (e.g., switching between entities), pass a `key` derived from that prop. React unmounts and remounts the component, clearing all state. Do not use useEffect to manually reset state variables.

**Incorrect (effect-driven reset):**

```tsx
function EditContact({ contact }: { contact: Contact }) {
  const [name, setName] = useState(contact.name)
  const [email, setEmail] = useState(contact.email)

  useEffect(() => {
    setName(contact.name)
    setEmail(contact.email)
  }, [contact])

  return (
    <>
      <input value={name} onChange={e => setName(e.target.value)} />
      <input value={email} onChange={e => setEmail(e.target.value)} />
    </>
  )
}
```

This renders once with stale state, then re-renders after the effect fires. Every piece of state inside the component needs a corresponding reset in the effect.

**Correct (key-based reset):**

```tsx
function EditContactWrapper({ contact }: { contact: Contact }) {
  return <EditContact contact={contact} key={contact.id} />
}

function EditContact({ contact }: { contact: Contact }) {
  const [name, setName] = useState(contact.name)
  const [email, setEmail] = useState(contact.email)

  return (
    <>
      <input value={name} onChange={e => setName(e.target.value)} />
      <input value={email} onChange={e => setEmail(e.target.value)} />
    </>
  )
}
```

React treats components with different keys as entirely different instances. All state resets automatically, including nested child state.

References: [You Might Not Need an Effect - Resetting all state when a prop changes](https://react.dev/learn/you-might-not-need-an-effect#resetting-all-state-when-a-prop-changes)
