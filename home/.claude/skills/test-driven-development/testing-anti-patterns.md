# Testing Anti-Patterns

**Load this reference when:** writing or changing tests, adding mocks, or tempted to add test-only methods to production code.

These rules apply regardless of framework (Jest/Vitest, pytest, Go interfaces, etc.).

## Overview

Tests must verify real behavior, not mock behavior. Mocks are a means to isolate, not the thing being tested.

**Core principle:** Test what the code does, not what the mocks do.

**Following strict TDD prevents these anti-patterns.**

## The Iron Laws

```
1. NEVER test mock behavior
2. NEVER add test-only methods to production classes
3. NEVER mock without understanding dependencies
```

## Anti-Pattern 1: Testing Mock Behavior

**The violation:**
```text
# BAD: Testing that the mock exists
test "renders sidebar":
    render Page with Sidebar mocked
    assert find_by_test_id("sidebar-mock")
```

**Why this is wrong:**
- You are verifying the mock works, not that the component works
- Test passes when mock is present, fails when it is not
- Tells you nothing about real behavior

**your human partner's correction:** "Are we testing the behavior of a mock?"

**The fix:**
```text
# GOOD: Test real behavior or do not mock it
test "renders sidebar":
    render Page with real Sidebar
    assert page_has_navigation()

# OR if sidebar must be mocked for isolation:
# Do not assert on the mock - test Page's behavior with sidebar present
```

### Gate Function

```
BEFORE asserting on any mock element:
  Ask: "Am I testing real component behavior or just mock existence?"

  IF testing mock existence:
    STOP - Delete the assertion or unmock the component

  Test real behavior instead
```

## Anti-Pattern 2: Test-Only Methods in Production

**The violation:**
```text
# BAD: destroy() only used in tests
class Session:
    def destroy(self):  # Looks like production API
        workspace_manager.destroy(self.id)
        # ... cleanup

# In tests
after_each():
    session.destroy()
```

**Why this is wrong:**
- Production class polluted with test-only code
- Dangerous if accidentally called in production
- Violates YAGNI and separation of concerns
- Confuses object lifecycle with entity lifecycle

**The fix:**
```text
# GOOD: Test utilities handle test cleanup
# Session has no destroy() - it is stateless in production

# In test-utils/
function cleanup_session(session):
    workspace = session.get_workspace_info()
    if workspace:
        workspace_manager.destroy(workspace.id)

# In tests
after_each():
    cleanup_session(session)
```

### Gate Function

```
BEFORE adding any method to production class:
  Ask: "Is this only used by tests?"

  IF yes:
    STOP - Do not add it
    Put it in test utilities instead

  Ask: "Does this class own this resource's lifecycle?"

  IF no:
    STOP - Wrong class for this method
```

## Anti-Pattern 3: Mocking Without Understanding

**The violation:**
```text
# BAD: Mock breaks test logic
test "detects duplicate server":
    mock ToolCatalog.discover_and_cache_tools() -> no-op

    add_server(config)
    add_server(config)  # Should throw - but will not
```

**Why this is wrong:**
- Mocked method had side effect test depended on (writing config)
- Over-mocking to be safe breaks actual behavior
- Test passes for wrong reason or fails mysteriously

**The fix:**
```text
# GOOD: Mock at correct level
test "detects duplicate server":
    mock slow_server_startup()  # Keep config writes real

    add_server(config)
    add_server(config)  # Duplicate detected
```

### Gate Function

```
BEFORE mocking any method:
  STOP - Do not mock yet

  1. Ask: "What side effects does the real method have?"
  2. Ask: "Does this test depend on any of those side effects?"
  3. Ask: "Do I fully understand what this test needs?"

  IF depends on side effects:
    Mock at lower level (the actual slow or external operation)
    OR use test doubles that preserve necessary behavior
    NOT the high-level method the test depends on

  IF unsure what test depends on:
    Run test with real implementation FIRST
    Observe what actually needs to happen
    THEN add minimal mocking at the right level

  Red flags:
    - "I will mock this to be safe"
    - "This might be slow, better mock it"
    - Mocking without understanding the dependency chain
```

## Anti-Pattern 4: Incomplete Mocks

**The violation:**
```text
# BAD: Partial mock - only fields you think you need
mock_response = {
    status: "success",
    data: { user_id: "123", name: "Alice" }
    # Missing metadata that downstream code uses
}

# Later: breaks when code accesses response.metadata.request_id
```

**Why this is wrong:**
- Partial mocks hide structural assumptions
- Downstream code may depend on fields you did not include
- Tests pass but integration fails
- False confidence

**The Iron Rule:** Mock the complete data structure as it exists in reality, not just fields your immediate test uses.

**The fix:**
```text
# GOOD: Mirror real API completeness
mock_response = {
    status: "success",
    data: { user_id: "123", name: "Alice" },
    metadata: { request_id: "req-789", timestamp: 1234567890 }
    # All fields real API returns
}
```

### Gate Function

```
BEFORE creating mock responses:
  Check: "What fields does the real API response contain?"

  Actions:
    1. Examine actual API response from docs or examples
    2. Include ALL fields system might consume downstream
    3. Verify mock matches real response schema completely

  Critical:
    If you are creating a mock, you must understand the ENTIRE structure
    Partial mocks fail silently when code depends on omitted fields

  If uncertain: Include all documented fields
```

## Anti-Pattern 5: Integration Tests as Afterthought

**The violation:**
```
Implementation complete
No tests written
"Ready for testing"
```

**Why this is wrong:**
- Testing is part of implementation, not optional follow-up
- TDD would have caught this
- Cannot claim complete without tests

**The fix:**
```
TDD cycle:
1. Write failing test
2. Implement to pass
3. Refactor
4. THEN claim complete
```

## When Mocks Become Too Complex

**Warning signs:**
- Mock setup longer than test logic
- Mocking everything to make test pass
- Mocks missing methods real components have
- Test breaks when mock changes

**your human partner's question:** "Do we need to be using a mock here?"

**Consider:** Integration tests with real components often simpler than complex mocks

## TDD Prevents These Anti-Patterns

**Why TDD helps:**
1. **Write test first** -> Forces you to think about what you are actually testing
2. **Watch it fail** -> Confirms test tests real behavior, not mocks
3. **Minimal implementation** -> No test-only methods creep in
4. **Real dependencies** -> You see what the test actually needs before mocking

**If you are testing mock behavior, you violated TDD** - you added mocks without watching test fail against real code first.

## Quick Reference

| Anti-Pattern | Fix |
|--------------|-----|
| Assert on mock elements | Test real behavior or unmock it |
| Test-only methods in production | Move to test utilities |
| Mock without understanding | Understand dependencies first, mock minimally |
| Incomplete mocks | Mirror real API completely |
| Tests as afterthought | TDD - tests first |
| Over-complex mocks | Consider integration tests |

## Red Flags

- Assertion checks for `*-mock` test IDs
- Methods only called in test files
- Mock setup is >50% of test
- Test fails when you remove mock
- Cannot explain why mock is needed
- Mocking "just to be safe"

## The Bottom Line

**Mocks are tools to isolate, not things to test.**

If TDD reveals you are testing mock behavior, you went wrong.

Fix: Test real behavior or question why you are mocking at all.
