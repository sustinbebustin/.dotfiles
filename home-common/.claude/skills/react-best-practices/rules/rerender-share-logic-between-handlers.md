---
title: Share Logic Between Event Handlers
impact: MEDIUM
impactDescription: prevents effect re-runs on unrelated state changes and duplicate side effects
tags: rerender, useEffect, event-handlers, shared-logic, side-effects
---

## Share Logic Between Event Handlers

When multiple event handlers need the same side effect (e.g., showing a notification), extract a shared function and call it from each handler. Do not model the action as a state change observed by an effect -- this causes the effect to fire on page load, refresh, and any unrelated state change that satisfies the condition.

**Incorrect (effect watches state for event-specific logic):**

```tsx
function ProductPage({ product, addToCart }: Props) {
  useEffect(() => {
    if (product.isInCart) {
      showNotification(`Added ${product.name} to cart!`)
    }
  }, [product])

  function handleBuyClick() {
    addToCart(product)
  }

  function handleCheckoutClick() {
    addToCart(product)
    navigateTo('/checkout')
  }
}
```

If the cart is persisted across page loads, `product.isInCart` is `true` on mount and the notification fires on every page visit.

**Correct (shared helper called from handlers):**

```tsx
function ProductPage({ product, addToCart }: Props) {
  function buyProduct() {
    addToCart(product)
    showNotification(`Added ${product.name} to cart!`)
  }

  function handleBuyClick() {
    buyProduct()
  }

  function handleCheckoutClick() {
    buyProduct()
    navigateTo('/checkout')
  }
}
```

The key question: should this code run because the component was **displayed**, or because the user **did something**? If it's a user action, it belongs in an event handler.

References: [You Might Not Need an Effect - Sharing logic between event handlers](https://react.dev/learn/you-might-not-need-an-effect#sharing-logic-between-event-handlers)
