---
name: rules-of-react
description: "React's 10 correctness rules: purity, immutability, and hooks call-site restrictions. These are the React Compiler's contract -- violating them causes bugs and Compiler bail-outs. Use when writing, reviewing, or refactoring any React component or custom hook."
---

# Rules of React

10 mandatory correctness rules from [react.dev/reference/rules](https://react.dev/reference/rules) (React v19). These are not guidelines -- they are the contract between your code and React's rendering engine. The React Compiler assumes all 10 are followed; violations cause silent bail-outs or incorrect optimizations.

## Relationship to `react-best-practices`

- **rules-of-react** = correctness (mandatory; the app breaks if violated)
- **react-best-practices** = performance (graded; the app is slower if ignored)

## Quick Reference

| #  | Rule                                         | Category      | Violation Consequence                    |
|----|----------------------------------------------|---------------|------------------------------------------|
| 1  | Components must be idempotent                | Purity        | Unpredictable renders, Compiler bail-out |
| 2  | Side effects outside of render               | Purity        | Bugs on re-render, Compiler bail-out     |
| 3  | Props are immutable                          | Purity        | Inconsistent output across renders       |
| 4  | State is immutable (use setter)              | Purity        | UI won't update                          |
| 5  | Hook args/returns are immutable              | Purity        | Broken memoization and caching           |
| 6  | Values immutable after JSX usage             | Purity        | Stale JSX from eager evaluation          |
| 7  | Never call components as functions           | React Control | No lifecycle, hooks, or reconciliation   |
| 8  | Never pass hooks as regular values           | React Control | Breaks hook call-site tracking           |
| 9  | Hooks at top level only                      | Hooks         | State corruption from shifted positions  |
| 10 | Hooks only from React functions              | Hooks         | React can't track state or effects       |

## Rule Categories

### Category 1: Components and Hooks Must Be Pure (Rules 1-6)

Purity makes code predictable and allows React to safely re-render components multiple times. React may render a component more than once to produce the best user experience -- purity is what makes this safe. The Compiler relies on purity to insert automatic memoization.

**Key principle:** Given the same inputs (props, state, context), a component must always return the same output. Local mutation of freshly created values is fine.

### Category 2: React Calls Components and Hooks (Rules 7-8)

React is responsible for rendering components and executing hooks. Calling components as functions or passing hooks as values bypasses React's tree management, breaking reconciliation, lifecycle, and state tracking.

### Category 3: Rules of Hooks (Rules 9-10)

Hooks are tracked by call order. Conditional or nested hook calls shift positions and corrupt state. Hooks must only be called from React function components or custom hooks (functions starting with `use`).

## Full Rules, Enforcement, and Compiler Bail-Outs

[AGENTS.md](AGENTS.md) holds the complete rules with correct/incorrect code examples, the "You Might Not Need an Effect" guidance, the `eslint-plugin-react-hooks` enforcement table, `<StrictMode>` behavior, and known React Compiler bail-out patterns (such as branching expressions inside `try/catch`).
