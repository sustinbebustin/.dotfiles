# Rules of React

**Source:** [react.dev/reference/rules](https://react.dev/reference/rules) (React v19)

React's 10 mandatory correctness rules. These are the contract between your code and React's rendering engine. The React Compiler assumes all 10 are followed.

---

## Category 1: Components and Hooks Must Be Pure

Purity makes code predictable, debuggable, and safe for React to re-render multiple times. The Compiler inserts automatic memoization based on the assumption of purity.

### Rule 1 -- Components Must Be Idempotent

Given the same inputs (props, state, context), a component must always return the same output. Non-deterministic functions like `new Date()` or `Math.random()` must not run during render.

```jsx
// WRONG: different result every render
function Clock() {
  const time = new Date();
  return <span>{time.toLocaleString()}</span>;
}

// RIGHT: non-idempotent code in effect
function Clock() {
  const [time, setTime] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(id);
  }, []);

  return <span>{time.toLocaleString()}</span>;
}
```

### Rule 2 -- Side Effects Must Run Outside of Render

Side effects (DOM manipulation, data fetching, logging) must never run during render. Use event handlers or `useEffect`.

```jsx
// WRONG: side effect during render
function ProductDetailPage({ product }) {
  document.title = product.title;
}

// RIGHT: side effect in useEffect
function ProductDetailPage({ product }) {
  useEffect(() => {
    document.title = product.title;
  }, [product.title]);
}
```

**Exception -- local mutation is fine:**

```jsx
function FriendList({ friends }) {
  const items = []; // created locally, never leaks
  for (let i = 0; i < friends.length; i++) {
    items.push(<Friend key={friends[i].id} friend={friends[i]} />);
  }
  return <section>{items}</section>;
}
```

This is safe because `items` is created fresh every render and never escapes the component.

### Rule 3 -- Props Are Immutable

Never mutate props. If you need a modified version, create a copy.

```jsx
// WRONG: mutating props
function Post({ item }) {
  item.url = new Url(item.url, base);
  return <Link url={item.url}>{item.title}</Link>;
}

// RIGHT: creating a new value
function Post({ item }) {
  const url = new Url(item.url, base);
  return <Link url={url}>{item.title}</Link>;
}
```

### Rule 4 -- State Is Immutable

Never assign to state variables directly. Always use the setter function from `useState`.

```jsx
// WRONG: direct mutation (UI won't update)
function Counter() {
  const [count, setCount] = useState(0);
  function handleClick() {
    count = count + 1;
  }
}

// RIGHT: using the setter
function Counter() {
  const [count, setCount] = useState(0);
  function handleClick() {
    setCount(count + 1);
  }
}
```

### Rule 5 -- Hook Arguments and Return Values Are Immutable

Once values are passed to a hook, don't modify them. Hooks may memoize based on those arguments, so mutations silently break caching.

```jsx
// WRONG: mutating hook arguments
function useIconStyle(icon) {
  const theme = useContext(ThemeContext);
  if (icon.enabled) {
    icon.className = computeStyle(icon, theme);
  }
  return icon;
}

// RIGHT: making a copy
function useIconStyle(icon) {
  const theme = useContext(ThemeContext);
  const newIcon = { ...icon };
  if (icon.enabled) {
    newIcon.className = computeStyle(icon, theme);
  }
  return newIcon;
}
```

### Rule 6 -- Values Are Immutable After Being Passed to JSX

Don't mutate objects after they've been used in a JSX expression. React may evaluate JSX eagerly, so later mutations won't be reflected. Move mutations before JSX creation.

```jsx
// WRONG: mutating after JSX usage
function Page({ colour }) {
  const styles = { colour, size: "large" };
  const header = <Header styles={styles} />;
  styles.size = "small"; // too late -- already used above
  const footer = <Footer styles={styles} />;
  return <>{header}<Content />{footer}</>;
}

// RIGHT: separate values
function Page({ colour }) {
  const headerStyles = { colour, size: "large" };
  const header = <Header styles={headerStyles} />;
  const footerStyles = { colour, size: "small" };
  const footer = <Footer styles={footerStyles} />;
  return <>{header}<Content />{footer}</>;
}
```

---

## Category 2: React Calls Components and Hooks

React is declarative. You tell React what to render; React figures out how and when.

### Rule 7 -- Never Call Component Functions Directly

Components should only be used in JSX. Calling them as functions bypasses React's tree management.

```jsx
// WRONG: calling as a function
function App() {
  return BlogPost();
}

// RIGHT: using in JSX
function App() {
  return <BlogPost />;
}
```

When called directly, React doesn't create a node in the component tree -- no hooks, no lifecycle, no reconciliation.

### Rule 8 -- Never Pass Hooks Around as Regular Values

Hooks should only be called inside components or custom hooks. Never store them in variables, pass them as arguments, or call them dynamically.

```jsx
// WRONG: dynamically selecting a hook
function ChatInput() {
  const useSettings = getSettings ? useDesktopSettings : useMobileSettings;
  const settings = useSettings();
}

// RIGHT: call both, select result
function ChatInput() {
  const desktopSettings = useDesktopSettings();
  const mobileSettings = useMobileSettings();
  const settings = getSettings ? desktopSettings : mobileSettings;
}
```

---

## Category 3: Rules of Hooks

### Rule 9 -- Only Call Hooks at the Top Level

Don't call hooks inside loops, conditions, or nested functions. Always call hooks at the top level of your React function, before any early returns.

```jsx
// WRONG: hook inside a condition
function Form() {
  const [name, setName] = useState('Mary');

  if (name !== '') {
    useEffect(function persistForm() {
      localStorage.setItem('formData', name);
    });
  }

  const [surname, setSurname] = useState('Poppins');
}

// RIGHT: condition inside the hook
function Form() {
  const [name, setName] = useState('Mary');

  useEffect(function persistForm() {
    if (name !== '') {
      localStorage.setItem('formData', name);
    }
  });

  const [surname, setSurname] = useState('Poppins');
}
```

React tracks hooks by call order. If a hook is conditionally skipped, every subsequent hook shifts position and returns the wrong state.

### Rule 10 -- Only Call Hooks from React Functions

Don't call hooks from regular JavaScript functions. Call them only from:
- React function components
- Custom hooks (functions whose name starts with `use`)

```jsx
// WRONG: hook in a regular function
function calculateTotal(items) {
  const [total, setTotal] = useState(0);
}

// RIGHT: hook in a custom hook
function useTotal(items) {
  const [total, setTotal] = useState(0);
  // ...
  return total;
}
```

---

## Enforcement

### ESLint (Compile-Time)

`eslint-plugin-react-hooks` v7+ includes Compiler-powered rules that enforce these correctness constraints:

| Rule                              | Catches                                   | Rules Enforced |
|-----------------------------------|-------------------------------------------|----------------|
| `react-hooks/rules-of-hooks`     | Hooks called conditionally or in loops    | 9, 10          |
| `react-hooks/exhaustive-deps`    | Missing or extra effect dependencies      | 2              |
| `react-hooks/purity`             | Impure code during render                 | 1, 2           |
| `react-hooks/immutability`       | Mutating props, state, or hook values     | 3, 4, 5, 6     |
| `react-hooks/refs`               | Accessing ref `.current` during render    | 2              |
| `react-hooks/set-state-in-render`| Calling `setState` during render          | 2              |

The `recommended-latest` preset enables all of these:

```js
// eslint.config.mjs
import reactHooks from 'eslint-plugin-react-hooks'

export default [
  reactHooks.configs.flat['recommended-latest'],
]
```

### StrictMode (Runtime)

`<StrictMode>` renders components twice in development to surface impure render logic (Rules 1-6). Effects also double-fire to catch missing cleanup.

### React Compiler (Build-Time)

The Compiler statically analyzes components and inserts memoization. It assumes all 10 rules are followed. Violations cause:
- **Silent skip**: Component left un-optimized (default behavior)
- **Build error**: When `panicThreshold: "all_errors"` is configured
- **Incorrect output**: If a violation slips past static analysis

#### Known Bail-Out Patterns (v1.0.0)

The Compiler bails out on these code patterns even if they don't violate the rules:
- `try/finally` blocks (any finalizer clause)
- `try` without `catch`
- Any "value block" expression inside a `try/catch` body

**Value block expressions** are expressions that produce branching control flow in the Compiler's HIR (High-level Intermediate Representation). The Compiler inserts `maybe-throw` terminals after every instruction inside `try/catch`, and cannot yet convert `maybe-throw` terminals inside value block chains.

Expressions that bail out inside `try/catch`:
- `?.` (optional chaining)
- `? :` (ternary/conditional expressions)
- `&&`, `||`, `??` (logical expressions)

Expressions that are safe inside `try/catch`:
- `obj.prop` (regular member access -- simple instruction, no value block)
- `fn()` (regular function call -- simple instruction)
- `if/else` statements (standard control flow, not value blocks)

**Workaround:** Hoist branching expressions above the `try` block. Use `if` guards (safe) instead of `?.` or ternaries (bail out).

```jsx
// WRONG: optional chain inside try causes Compiler bail-out
function Component({ ref }) {
  const save = () => {
    try {
      ref.current?.save();
      doWork();
    } catch (e) {
      handleError(e);
    }
  };
}

// RIGHT: hoist to variable, guard with if (not ?.) outside try
function Component({ ref }) {
  const save = () => {
    const target = ref.current;
    if (target) {
      try {
        target.save();
        doWork();
      } catch (e) {
        handleError(e);
      }
    }
  };
}
```

#### Diagnostics

Set `panicThreshold` to surface all Compiler bail-outs during development:

```ts
// next.config.ts
const nextConfig = {
  reactCompiler: {
    panicThreshold: "all_errors", // throws on every bail-out
  },
};
```

This makes the Compiler throw instead of silently skipping, surfacing issues early.
