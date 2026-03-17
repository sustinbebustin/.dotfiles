---
description: Print the Impeccable design commands cheatsheet, optionally recommend commands for a goal
argument-hint: "[optional: describe what you want to achieve]"
disable-model-invocation: true
model: sonnet
---

Print the following cheatsheet exactly as shown:

---

# Impeccable Commands

## Diagnostic

- `/audit` -- Perform comprehensive audit of interface quality across accessibility, performance, theming, and responsive design. Generates detailed report of issues with severity ratings and recommendations.
  - Leads to: `/normalize`, `/harden`, `/optimize`, `/adapt`, `/clarify`
- `/critique` -- Evaluate design effectiveness from a UX perspective. Assesses visual hierarchy, information architecture, emotional resonance, and overall design quality with actionable feedback.
  - Leads to: `/polish`, `/distill`, `/bolder`, `/quieter`, `/typeset`, `/arrange`

## Quality

- `/harden` -- Improve interface resilience through better error handling, i18n support, text overflow handling, and edge case management. Makes interfaces robust and production-ready.
  - Combines with: `/optimize`
- `/normalize` -- Normalize design to match your design system and ensure consistency.
  - Combines with: `/clarify`, `/adapt`
- `/optimize` -- Improve interface performance across loading speed, rendering, animations, images, and bundle size. Makes experiences faster and smoother.
- `/polish` -- Final quality pass before shipping. Fixes alignment, spacing, consistency, and detail issues that separate good from great.

## Intensity

- `/bolder` -- Amplify safe or boring designs to make them more visually interesting and stimulating. Increases impact while maintaining usability.
  - Pairs with: `/quieter`
- `/quieter` -- Tone down overly bold or visually aggressive designs. Reduces intensity while maintaining design quality and impact.
  - Pairs with: `/bolder`

## Adaptation

- `/adapt` -- Adapt designs to work across different screen sizes, devices, contexts, or platforms. Ensures consistent experience across varied environments.
  - Combines with: `/normalize`, `/clarify`
- `/clarify` -- Improve unclear UX copy, error messages, microcopy, labels, and instructions. Makes interfaces easier to understand and use.
  - Combines with: `/normalize`, `/adapt`
- `/distill` -- Strip designs to their essence by removing unnecessary complexity. Great design is simple, powerful, and clean.
  - Combines with: `/quieter`, `/normalize`

## Enhancement

- `/animate` -- Review a feature and enhance it with purposeful animations, micro-interactions, and motion effects that improve usability and delight.
  - Combines with: `/delight`
- `/arrange` -- Improve layout, spacing, and visual rhythm. Fixes monotonous grids, inconsistent spacing, and weak visual hierarchy to create intentional compositions.
  - Combines with: `/distill`, `/adapt`
- `/colorize` -- Add strategic color to features that are too monochromatic or lack visual interest. Makes interfaces more engaging and expressive.
  - Combines with: `/bolder`, `/delight`
- `/delight` -- Add moments of joy, personality, and unexpected touches that make interfaces memorable and enjoyable to use. Elevates functional to delightful.
  - Combines with: `/bolder`, `/animate`
- `/onboard` -- Design or improve onboarding flows, empty states, and first-time user experiences. Helps users get started successfully and understand value quickly.
  - Combines with: `/clarify`, `/delight`
- `/typeset` -- Improve typography by fixing font choices, hierarchy, sizing, weight consistency, and readability. Makes text feel intentional and polished.
  - Combines with: `/bolder`, `/normalize`
- `/overdrive` BETA -- Push interfaces past conventional limits with technically ambitious implementations. Whether that's a shader, a 60fps virtual table, spring physics on a dialog, or scroll-driven reveals -- make users ask "how did they do that?"
  - Combines with: `/animate`, `/delight`

## System

- `/extract` -- Extract and consolidate reusable components, design tokens, and patterns into your design system. Identifies opportunities for systematic reuse and enriches your component library.
- `/teach-impeccable` -- One-time setup that gathers design context for your project and saves it to your AI config file. Run once to establish persistent design guidelines.

---

If the user provided an argument describing what they want to achieve, recommend which Impeccable command(s) to use and in what order. Explain briefly why each recommended command fits their goal. Suggest a workflow if multiple commands should be chained. Keep the recommendation concise.

User's goal: $ARGUMENTS
