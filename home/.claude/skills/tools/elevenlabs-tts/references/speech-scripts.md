# Writing speech scripts

A listener gets one pass, at the speaker's pace, with no headings, bold, or tables to lean on. A script carries that structure in its words. Rewrite source text as a script; never pipe prose, markdown, or HTML text straight in.

## Shape for the ear

- **Signpost.** Say the structure out loud: "Three things decide this. First... Second... And last..." Name the takeaway before the detail, then land on it again at the end; the ear cannot skim back.
- **One idea per sentence.** 10-20 words, subject and verb early. Break nested clauses and parentheticals into their own sentences; brackets and asides are read flat and lose the thread.
- **Concrete over abstract.** One worked example ("a Utah proposal created October first") lands faster than the general rule; give the example, then the rule.
- **Conversational register.** Contractions, "you", plain verbs. The tone of a colleague explaining at a whiteboard, not a document being read.
- **Pacing.** A paragraph break is a breath. An ellipsis (`...`) adds a weighted pause; an em-dash (`--`) a short one. CAPS stresses one word; use it a few times per clip at most.

## Say it the way it should sound

Write every token as the words to be spoken. Built-in normalization guesses, and guesses differ by model and language.

| Written | Script |
|---------|--------|
| `$4,500`, `$12 / lf` | "forty-five hundred dollars", "twelve dollars a linear foot" |
| `Oct 31`, `2026-10-01` | "October thirty-first", "October first, twenty twenty-six" |
| `30%`, `3.5`, `2nd` | "thirty percent", "three point five", "second" |
| `->`, `!=`, `>=`, `/` | "becomes" / "leads to", "is not equal to", "at least", "or" / "per" |
| `SelectionUnavailable`, `pinnedAt` | "selection unavailable", "pinned at" -- or say what it means |
| `src/app/page.tsx`, URLs | what the file does ("the proposal page"); spell a path only if the listener must type it |
| Acronyms | spelled when said as letters ("S-K-U" or "skew" -- pick how the team says it), expanded when the expansion is clearer |
| `e.g.`, `i.e.`, `vs.` | "for example", "that is", "versus" |
| Emoji, bullets, `**`, `#` | removed |

**Pronunciation.** For a name the model gets wrong: on `eleven_v4`, write IPA between slashes inside quotes (`"/ˈnɪkəl/"`); on other models respell it phonetically ("Claughton" -> "Cloffton"). Test important names once before generating a long set.

## Delivery controls (eleven_v4 / eleven_v3)

- SSML `<break>` tags do nothing on v4 and v3; use punctuation and paragraph breaks for pauses.
- Audio tags in square brackets (`[thoughtful]`, `[warmly]`, `[short pause]`) steer delivery. For explanation, use few or none: a tag at a genuine turn ("[slower, emphatic] This is the part that matters.") helps, a tag per sentence makes it a performance. Describe the voice quality, not a sound -- an ambiguous tag can come out as a sound effect.
- Never put stage directions in plain text ("she said warmly"); the model reads them aloud.
- v4 has no speed setting. Pace comes from the writing; listeners speed up playback themselves.

## Length

About 150 words or 900 characters per minute. A focused clip runs 30-120 seconds; a whole-topic overview stays under five minutes. Past that, split into clips the listener can choose between.

## Ground truth

The script states only what the source states. Numbers, names, and outcomes are copied exactly, then converted to spoken form. When the source changes, the audio is stale -- regenerate the affected clips.
