---
title: 'You Might Not Need an Effect'
source: https://react.dev/learn/you-might-not-need-an-effect
---

# You Might Not Need an Effect

Effects are an escape hatch from the React paradigm. They let you "step outside" of React and synchronize your components with some external system like a non-React widget, network, or the browser DOM. If there is no external system involved (for example, if you want to update a component's state when some props or state change), you shouldn't need an Effect. Removing unnecessary Effects will make your code easier to follow, faster to run, and less error-prone.

## Two cases where you don't need Effects

1. **Transforming data for rendering.** Transform data at the top level of your component. It re-runs automatically when props or state change. Using Effect + setState causes an extra render pass with stale values.
2. **Handling user events.** In an event handler you know exactly what the user did. By the time an Effect runs, you've lost that context. Use event handlers for event-specific logic.

You DO need Effects to synchronize with external systems (third-party widgets, browser APIs, network). Modern frameworks provide built-in data fetching that's preferred over raw Effects.

---

## Anti-patterns and corrections

### 1. Derived state from props or state

When something can be calculated from existing props or state, don't put it in state. Calculate it during rendering.

```jsx
// WRONG: redundant state and unnecessary Effect
const [fullName, setFullName] = useState('');
useEffect(() => {
  setFullName(firstName + ' ' + lastName);
}, [firstName, lastName]);

// RIGHT: calculated during rendering
const fullName = firstName + ' ' + lastName;
```

### 2. Caching expensive calculations

Use `useMemo`, not Effect + setState. The React Compiler can also auto-memoize in many cases.

```jsx
// WRONG
const [visibleTodos, setVisibleTodos] = useState([]);
useEffect(() => {
  setVisibleTodos(getFilteredTodos(todos, filter));
}, [todos, filter]);

// RIGHT
const visibleTodos = useMemo(
  () => getFilteredTodos(todos, filter),
  [todos, filter]
);
```

If `getFilteredTodos()` is fast (not looping over thousands of objects), you can skip `useMemo` entirely and just compute inline.

### 3. Resetting all state when a prop changes

Use `key` to tell React to treat components with different identities as completely separate instances. This resets all state in the subtree.

```jsx
// WRONG: resetting state in an Effect
export default function ProfilePage({ userId }) {
  const [comment, setComment] = useState('');
  useEffect(() => {
    setComment('');
  }, [userId]);
  // ...
}

// RIGHT: use key to reset state
export default function ProfilePage({ userId }) {
  return <Profile userId={userId} key={userId} />;
}

function Profile({ userId }) {
  const [comment, setComment] = useState(''); // resets automatically on key change
  // ...
}
```

### 4. Adjusting some state when a prop changes

Prefer deriving the value during render. If you must adjust state, the `prevItems` pattern (setting state during render) is better than an Effect, but usually indicates the data model can be improved.

```jsx
// WRONG
useEffect(() => {
  setSelection(null);
}, [items]);

// ACCEPTABLE (but usually avoidable)
const [prevItems, setPrevItems] = useState(items);
if (items !== prevItems) {
  setPrevItems(items);
  setSelection(null);
}

// BEST: store ID, derive the object
const [selectedId, setSelectedId] = useState(null);
const selection = items.find(item => item.id === selectedId) ?? null;
```

The "best" approach means if the item is in the list it stays selected; if removed, selection becomes `null` -- no reset logic needed.

### 5. Sharing logic between event handlers

If logic runs because the user did something, put it in event handlers, not Effects. Extract a shared function if multiple handlers need it.

```jsx
// WRONG: event-specific logic in an Effect
useEffect(() => {
  if (product.isInCart) {
    showNotification(`Added ${product.name} to the shopping cart!`);
  }
}, [product]);

// RIGHT: shared function called from event handlers
function buyProduct() {
  addToCart(product);
  showNotification(`Added ${product.name} to the shopping cart!`);
}
function handleBuyClick() { buyProduct(); }
function handleCheckoutClick() { buyProduct(); navigateTo('/checkout'); }
```

The Effect version is buggy: it fires on page reload if the product was already in the cart.

### 6. Sending a POST request

The analytics POST (fires because component was displayed) belongs in an Effect. The form submission POST (fires because user clicked Submit) belongs in an event handler.

```jsx
// RIGHT: analytics in Effect, submission in event handler
function Form() {
  useEffect(() => {
    post('/analytics/event', { eventName: 'visit_form' });
  }, []);

  function handleSubmit(e) {
    e.preventDefault();
    post('/api/register', { firstName, lastName });
  }
}
```

Decision test: if this logic is caused by a particular interaction, keep it in the event handler. If it's caused by the user *seeing* the component on screen, keep it in the Effect.

### 7. Chains of computations

Effects that trigger each other via setState cause cascading re-renders. Calculate what you can during rendering, and compute the rest in event handlers.

```jsx
// WRONG: chain of Effects
useEffect(() => { if (card?.gold) setGoldCardCount(c => c + 1); }, [card]);
useEffect(() => { if (goldCardCount > 3) { setRound(r => r + 1); setGoldCardCount(0); } }, [goldCardCount]);
useEffect(() => { if (round > 5) setIsGameOver(true); }, [round]);

// RIGHT: derive during render + compute in event handler
const isGameOver = round > 5;

function handlePlaceCard(nextCard) {
  if (isGameOver) throw Error('Game already ended.');
  setCard(nextCard);
  if (nextCard.gold) {
    if (goldCardCount < 3) {
      setGoldCardCount(goldCardCount + 1);
    } else {
      setGoldCardCount(0);
      setRound(round + 1);
      if (round === 5) alert('Good game!');
    }
  }
}
```

Exception: chains of Effects are appropriate when each Effect synchronizes with a different external system (e.g., cascading dropdowns where options depend on a network response).

### 8. Initializing the application

Logic that should run once per app load (not once per mount) should use a module-level guard or run at module scope. Effects run twice in StrictMode development, which can break non-idempotent operations like auth token invalidation.

```jsx
// Option A: guard variable
let didInit = false;

function App() {
  useEffect(() => {
    if (!didInit) {
      didInit = true;
      loadDataFromLocalStorage();
      checkAuthToken();
    }
  }, []);
}

// Option B: module-level initialization (preferred for truly one-time logic)
if (typeof window !== 'undefined') {
  checkAuthToken();
  loadDataFromLocalStorage();
}

function App() { /* ... */ }
```

Keep app-wide initialization in root component modules or the application entry point.

### 9. Notifying parent components about state changes

Call the parent callback alongside setState in the same event handler. React batches updates from different components, so there's only one render pass.

```jsx
// WRONG: notifying parent in an Effect (runs too late, extra render)
useEffect(() => {
  onChange(isOn);
}, [isOn, onChange]);

// RIGHT: both updates in the event handler
function updateToggle(nextIsOn) {
  setIsOn(nextIsOn);
  onChange(nextIsOn);
}
```

Even better: consider whether the component should be fully controlled by the parent (no internal state; parent owns `isOn` and passes `onChange`). This is "lifting state up."

### 10. Passing data to the parent

When a child fetches data and pushes it to the parent via an Effect, the data flow becomes hard to trace. Let the parent fetch the data and pass it down.

```jsx
// WRONG
function Child({ onFetched }) {
  const data = useSomeAPI();
  useEffect(() => {
    if (data) onFetched(data);
  }, [onFetched, data]);
}

// RIGHT
function Parent() {
  const data = useSomeAPI();
  return <Child data={data} />;
}
```

### 11. Subscribing to an external store

Use `useSyncExternalStore` instead of manually managing subscriptions in an Effect. It's less error-prone and purpose-built for this use case.

```jsx
// WRONG: manual subscription
const [isOnline, setIsOnline] = useState(true);
useEffect(() => {
  function updateState() { setIsOnline(navigator.onLine); }
  updateState();
  window.addEventListener('online', updateState);
  window.addEventListener('offline', updateState);
  return () => {
    window.removeEventListener('online', updateState);
    window.removeEventListener('offline', updateState);
  };
}, []);

// RIGHT: useSyncExternalStore
function subscribe(callback) {
  window.addEventListener('online', callback);
  window.addEventListener('offline', callback);
  return () => {
    window.removeEventListener('online', callback);
    window.removeEventListener('offline', callback);
  };
}

function useOnlineStatus() {
  return useSyncExternalStore(
    subscribe,
    () => navigator.onLine,   // client
    () => true                 // server
  );
}
```

### 12. Fetching data

Data fetching in Effects is common but has pitfalls. You MUST add cleanup to avoid race conditions. Prefer framework-provided data fetching or libraries like SWR/React Query.

```jsx
// WRONG: no cleanup, race condition
useEffect(() => {
  fetchResults(query, page).then(json => {
    setResults(json);
  });
}, [query, page]);

// RIGHT: cleanup ignores stale responses
useEffect(() => {
  let ignore = false;
  fetchResults(query, page).then(json => {
    if (!ignore) setResults(json);
  });
  return () => { ignore = true; };
}, [query, page]);
```

When using Effects for data fetching, also consider: caching responses, server-side fetching for initial HTML, and avoiding network waterfalls.

Best practice: extract fetching into a custom Hook (`useData`) so raw `useEffect` calls don't proliferate across components.

---

## Decision heuristic

Ask: **"Why does this code need to run?"**

| Answer | Where to put it |
|--------|----------------|
| Because the user did something (clicked, submitted, dragged) | Event handler |
| Because the component appeared on screen, AND it involves an external system | Effect |
| Because some state or prop changed | Compute during render or `useMemo` |

## Recap

- If you can calculate something during render, you don't need an Effect.
- To cache expensive calculations, use `useMemo` instead of `useEffect`.
- To reset the state of an entire component tree, pass a different `key` to it.
- To reset a particular bit of state in response to a prop change, set it during rendering.
- Code that runs because a component was *displayed* should be in Effects; the rest should be in event handlers.
- If you need to update the state of several components, it's better to do it during a single event.
- Whenever you try to synchronize state variables in different components, consider lifting state up.
- You can fetch data with Effects, but you need to implement cleanup to avoid race conditions.
