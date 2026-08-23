---
name: better-error-messages
description: Standards for writing user-facing error messages (UI toasts, alerts, validation copy, error pages, API responses a user might see). Covers tone, jargon, blame, and the generic-vs-unclear distinction. Use when writing or reviewing any error string a user will read.
---

# Better Error Messages

User-facing error copy is part of the product. A bad error message is a bug -- it confuses the user, hides the real failure, blocks recovery, and trains people to ignore the next message they see. This skill defines the bar for any string a user will read on a failure path.

Out of scope: internal developer-facing errors (Go `error` values, thrown exceptions, structured log payloads). Those have a different audience and a different bar -- they *want* technical specificity, identifiers, and machine-parseable structure. See the global "Error Messages" rules for that surface.

## The Iron Rule

Every user-facing error must be able to answer five questions. If the message a user sees can't answer at least three of them, it isn't ready to ship.

1. **What happened?** What specifically did or did not occur.
2. **Why?** The cause, if known. "An issue on our end" is a valid answer when the cause is genuinely unknown.
3. **What's still safe?** What state was preserved -- draft saved, prior step intact, no charge applied. Silence here is read as "everything is lost."
4. **How do I recover?** The concrete next action the user can take. "Try again," "reconnect your account," "check your card details," with a path to that action.
5. **What if recovery fails?** A real way out -- a support channel, a help article, a contact path. Required when the user might not be able to fix it themselves, or when the failure might repeat.

A message that answers all five is rare and not required. A message that answers fewer than three is doing the user a disservice.

## The Four Anti-Patterns

These are the failure modes to flag and fix on sight. They are not stylistic; they are bugs.

### 1. Inappropriate tone

```
RED FLAGS

"Oops! Something went wrong."
"Whoops, that didn't work."
"Uh-oh -- looks like we hit a snag!"
"Yikes! Try again later."
```

Cutesy, casual, fluffy. The user is trying to complete something they care about -- submit a form, sign a contract, complete a payment. Treating their failed action like a sitcom mishap is condescending. Match the stakes: neutral, direct, respectful. Save the personality for the happy path.

### 2. Technical jargon

```
RED FLAGS

"We couldn't fetch your data."
"Request failed with status 503."
"Your credentials were denied."
"Null returned from upstream."
"Network error: ECONNREFUSED."
"Invalid token."
```

Internal vocabulary leaking into the product. Users don't fetch data, validate credentials, or parse tokens -- they sign in, save changes, send a message. Rewrite in the noun the user has in their head ("We couldn't load your saved drafts," "We couldn't sign you in"), and keep the technical detail for logs and dev tooling.

### 3. Passing the blame

```
RED FLAGS

"You entered an invalid value."             // blames the user
"Your card was declined by the issuer."     // blames the user's bank
"Stripe is not responding right now."       // blames a vendor by name
"The third-party API timed out."            // blames a vendor abstractly
```

Two flavors, both bad. Blaming the user makes them feel stupid even when it really was a typo -- shift the framing to the field and the fix ("This field needs a valid US ZIP code"). Blaming a third party is worse: the user came to *our* product and we own the surface. We can say "We're having trouble connecting to your bank" without naming the vendor and without shrugging.

### 4. Generic for no reason

```
RED FLAGS

"Something went wrong."
"An error occurred."
"Action could not be completed."
"Please try again."
```

If the call site knows the cause -- a validation failure, a rate limit, an expired session, a conflict, a missing dependency -- and the message still says "something went wrong," that is a choice to withhold information from the user. The catch-all generic message is reserved for the truly unknown path. Everywhere else, write to the actual cause.

## Generic vs Unclear -- Two Different Failures

It is tempting to lump every weak error message under "generic," but there is a second failure mode that needs the same urgency:

| Type | Example | The problem |
|------|---------|-------------|
| Generic | "Something went wrong and this action could not be completed." | Says nothing. The user has no idea what failed or what to do. |
| Unclear | "Make sure you allow the requested permissions and try again." | Says something, but in a way the user can't act on. Which permissions? Where? Allow them how? |

Both are equally bad. The fix is the same: figure out the actual cause and the actual recovery action, then write to that.

```
BETTER

Generic  -> "Your changes weren't saved. We had trouble reaching our servers.
            Your draft is still on this page -- try saving again in a moment.
            If this keeps happening, contact support."

Unclear  -> "We couldn't access your calendar. Open your browser's site
            settings, allow calendar access for this site, then try again."
```

## What a Good Message Looks Like

The same shape works across surfaces -- toast, modal, inline field error, full error page:

1. **State what happened.** A short, specific summary. "Unable to save your changes." "We couldn't process your payment." "Your session has expired."
2. **State why, when you know.** "Your card was declined." "The file is larger than the 25 MB limit." "We're having trouble connecting to your bank." When the cause is genuinely unknown: "due to an issue on our end" is honest. "Something went wrong" is not.
3. **Reassure when something was preserved.** "Your changes are still here as a draft." "We did not charge your card." "Your previous answers are saved." Silence on this point is read as loss.
4. **Give a concrete recovery action.** Not "try again later" with no detail -- "try again in a few minutes," "check your card details and resubmit," "reconnect your account from Settings > Integrations." Where space allows, the recovery action is a button or a link, not a sentence the user has to interpret.
5. **Give a way out.** When recovery might fail or isn't in the user's hands, link to a real channel: a help article, a support form, a phone number. "If the issue keeps happening, contact support" with no link is half a way out.

### A worked example

```
BAD

"Whoops! Something went wrong. The third-party you're trying to connect to
isn't responding, so we can't fetch your data. Try again later."

What's wrong:
- Tone ("Whoops!") trivializes a connection failure
- Jargon ("fetch your data")
- Blames a third party
- "Try again later" with no timing or alternative
- No reassurance about what state survived
```

```
GOOD

"Unable to connect your account.

Your changes were saved, but we could not connect your account due to a
technical issue on our end. Please try connecting again. If the issue
keeps happening, contact support."

Why it works:
- Direct, neutral tone
- States what happened ("unable to connect your account")
- States why honestly ("a technical issue on our end")
- Reassures ("your changes were saved")
- Concrete next step ("try connecting again")
- A way out ("contact support")
```

## Empathy Without Grovelling

The line is "speak to a friend who needs you to be useful, not a friend who needs to apologize."

- Default to neutral, direct, respectful. No "Sorry!" prefix on every message.
- Use "please" only when the situation warrants it -- a genuinely dire failure, or one we know we can't help the user solve from inside the product. A "please" on every error is noise; a "please" reserved for the hard cases lands.
- Never grovel. Long apologies waste the user's time and feel performative.
- Never joke. Even mild humor on a payment failure, a contract failure, or anything affecting the user's livelihood is misjudging the room.

## When You Genuinely Don't Know the Cause

Some failures arrive without a known cause -- a bare 5xx from a flaky upstream, an unhandled exception, a brand-new product where you haven't yet mapped the error surface. The honest response:

- Say what failed in user terms ("Your message wasn't sent").
- Say the cause is unknown in plain language ("due to an issue on our end"). Do not invent specificity you don't have -- false precision ("your card was declined for fraud") is worse than honest uncertainty.
- Reassure on preserved state.
- Provide the recovery action you can offer ("try again") and a way out ("contact support if this keeps happening").
- File a follow-up: add diagnostics so the next iteration of this message *can* be specific. A catch-all toast is a placeholder, not a destination.

## Translating Backend Failures Into User Copy

When the server already knows why something failed -- validation rejected a field, the rate limit fired, the session expired, a downstream provider is down, there is a conflict with existing data -- the user-facing surface should write to that specific case, not fall back to a generic "request failed."

Patterns to follow:

- Map server error codes to specific copy at the UI boundary. One toast string per known failure mode beats one toast string for "everything."
- Keep the catch-all toast for the genuinely unknown path. Even there, give a way out.
- Don't echo raw server messages or HTTP status codes into user-visible copy. Server messages are written for engineers; translate them.
- When the same backend error can mean different things in different UI contexts, the UI is the right place to choose the right phrasing -- the server doesn't know whether the user just tried to save, sign, send, or schedule.

## Detection Heuristics

When auditing existing strings, scan for these markers and challenge each one:

| Marker | Probable problem |
|--------|------------------|
| "Oops", "Whoops", "Uh-oh", "Yikes" | Inappropriate tone for a failure surface |
| "Something went wrong", "an error occurred" | Generic; almost always the call site knows more |
| "Try again later" with no detail | Lazy recovery; no timing, no alternative, no way out |
| "Please try again" with nothing else | Missing what / why / reassurance / way-out |
| "Failed to", "could not", "unable to" (alone) | Incomplete sentence -- says nothing about state, recovery, or way out |
| "Invalid", "bad request" | Jargon; doesn't tell the user which field or value to fix |
| "Fetch", "request", "endpoint", "upstream", "downstream" | Internal vocabulary leaking into the UI |
| HTTP codes ("503", "404", "500") in user-visible copy | Technical leakage; translate to the user's noun |
| Third-party vendor names in user-visible copy ("Stripe", "Plaid", "Twilio") | Blame-shifting; we own the surface |
| "You" + verb ("you entered", "you forgot") | Blames the user; reframe around the field or fix |
| Bare exclamation marks on error copy | Tone mismatch; failures aren't exciting |
| Long apology preamble ("We're so sorry...") | Grovelling; cut to what happened and what to do |
| Copy that doesn't mention what was preserved | Reads as "everything is lost"; add reassurance |
| Copy with no actionable next step | Failed the recovery test; add one or link to support |
| Copy that mentions a help article but no link | Half a way out; either link it or remove the reference |

## The Litmus Tests

Apply in order before shipping any user-facing error string:

1. **Cold-reader test.** Would the message make sense to a user who has never seen this product before, has no idea what we call our internal systems, and doesn't know the engineering plan? If no, rewrite.
2. **Specific-cause test.** Does the message describe the *actual* failure, or a generic stand-in? If the call site knows the cause, the message must reflect it.
3. **Reassurance test.** Does the message tell the user what state survived? If something *was* preserved and the message doesn't say so, the user assumes loss.
4. **Recovery test.** Is there a concrete next action the user can take? "Try again later" with no detail is not an action.
5. **False-precision test.** Does the message claim a cause we don't actually know? If we are guessing, say the cause is unknown -- don't invent one.
6. **Way-out test.** If recovery might not work, is there a real support channel, with a link or contact path? "Contact support" with no link is a half-answer.

A message that fails any of these is not pulling its weight. Rewrite or escalate.

## Push Back on "Just Make It Generic"

A common request: "We don't have time to figure out why this fails -- can you just add a generic error message here?" The honest answer is no, or at least "not as the destination."

- A generic message in production is a placeholder, not a feature. It comes with a follow-up: investigate the trigger, map the cause, replace the copy.
- "We don't know what causes this" is a *data* problem, not a *copy* problem. The fix is instrumentation, not vague language.
- A reviewer or writer pushed to add a "Something went wrong" string is entitled to ask: when does this fire, what does the server know at that point, and what should the user do next? If those three questions have answers, the message should reflect them.
