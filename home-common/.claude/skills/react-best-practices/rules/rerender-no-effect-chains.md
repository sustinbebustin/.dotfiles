---
title: Avoid Effect Chains
impact: MEDIUM
impactDescription: eliminates cascading re-renders and fragile state coupling
tags: rerender, useEffect, state, chains, cascading
---

## Avoid Effect Chains

Do not chain effects where each sets state that triggers the next. This causes cascading re-renders (one per effect in the chain) and creates brittle code that breaks when requirements change. Instead, derive what you can during render and compute next state in event handlers.

**Incorrect (chain of effects):**

```tsx
function Game() {
  const [card, setCard] = useState<Card | null>(null)
  const [goldCardCount, setGoldCardCount] = useState(0)
  const [round, setRound] = useState(1)
  const [isGameOver, setIsGameOver] = useState(false)

  useEffect(() => {
    if (card !== null && card.gold) {
      setGoldCardCount(c => c + 1)
    }
  }, [card])

  useEffect(() => {
    if (goldCardCount > 3) {
      setRound(r => r + 1)
      setGoldCardCount(0)
    }
  }, [goldCardCount])

  useEffect(() => {
    if (round > 5) {
      setIsGameOver(true)
    }
  }, [round])
}
```

Worst case: `setCard` -> render -> `setGoldCardCount` -> render -> `setRound` -> render -> `setIsGameOver` -> render. Four renders for one user action.

**Correct (derive + compute in handler):**

```tsx
function Game() {
  const [card, setCard] = useState<Card | null>(null)
  const [goldCardCount, setGoldCardCount] = useState(0)
  const [round, setRound] = useState(1)

  const isGameOver = round > 5

  function handlePlaceCard(nextCard: Card) {
    if (isGameOver) throw new Error('Game already ended.')

    setCard(nextCard)
    if (nextCard.gold) {
      if (goldCardCount < 3) {
        setGoldCardCount(goldCardCount + 1)
      } else {
        setGoldCardCount(0)
        setRound(round + 1)
      }
    }
  }
}
```

`isGameOver` is derived during render. All state transitions happen in a single event handler -- React batches the updates into one render.

References: [You Might Not Need an Effect - Chains of computations](https://react.dev/learn/you-might-not-need-an-effect#chains-of-computations)
