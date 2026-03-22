---
name: complete-next-task
description: Complete the next Overseer task
disable-model-invocation: true
argument-hint: [spec-path]
effort: max
---

First, invoke the skill tool to load the spec-planner skill:

```
skill({ name: 'overseer' })
```

Then follow the skill instructions to plan and implement the next available task. If a spec is attached, understand the context of the spec, but only implement the next task from overseer

<user-request>
$ARGUMENTS
</user-request>
