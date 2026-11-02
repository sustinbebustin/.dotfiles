---
name: elevenlabs-tts
description: Generate speech MP3s with ElevenLabs -- one clip from text, or a multi-clip narration stitched for consistent delivery. Use for any voiceover, audio version, or spoken explanation.
argument-hint: "<text or path to a text file> [output.mp3]"
allowed-tools: Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py *)
metadata:
  author: sustinbebustin
---

# ElevenLabs TTS

Turns text into MP3 using the voice and model in `${CLAUDE_SKILL_DIR}/.env`. Every character is billed, so generate only what was asked for, and regenerate single clips with `--only` rather than the whole set.

## Write for the ear first

Text written to be read sounds wrong spoken. Before generating anything longer than a sentence, rewrite it as a script per [speech-scripts.md](references/speech-scripts.md). A short phrase handed over verbatim goes as-is.

## One clip

Input is `$ARGUMENTS`: an existing file path means speak that file, otherwise the text itself. Output is the second argument, else `./speech.mp3` (pick a free name over clobbering).

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --text "<script>" --out <path>
uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --text-file <file> --out <path>   # long or quote-heavy text
```

## A narration (several clips)

For audio split into parts -- per section, per chapter -- write a manifest and generate the set in one run. Each clip is stitched to the ones before it (`previous_request_ids`) and told what follows (`next_text`), so the voice carries across clips instead of resetting.

```json
{"clips": [
  {"id": "overview", "title": "Overview", "script": "..."},
  {"id": "the-clock", "title": "The 30-day clock", "script": "..."}
]}
```

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --manifest <dir>/narration.json             # writes <dir>/01-overview.mp3, 02-the-clock.mp3
uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --manifest <dir>/narration.json --only the-clock   # redo one clip
```

Keep the manifest: it is the transcript and the source for regenerating. Clip ids are lowercase-hyphenated and the file numbering follows manifest order. Other keys on a clip are ignored, so a consumer (a page's player) can keep its own fields in the same file.

## Model and limits

The model comes from `ELEVENLABS_MODEL_ID`, else `eleven_v4`; pass `--model` only when the user names one. Per-request limits: 10,000 characters for `eleven_v4` and `eleven_multilingual_v2`, 5,000 for `eleven_v3`, 40,000 for `eleven_flash_v2_5`. A longer script becomes a narration of several clips; never truncate.

## Check the result

Each written clip prints `path  seconds  chars`. Speech runs about 15 characters a second; a clip far shorter than `chars / 15` was cut off and one far longer has run on or looped, so regenerate it with `--only`. On an error, relay the script's message verbatim -- a missing key means the user creates `${CLAUDE_SKILL_DIR}/.env` from `.env.example`.

Done when every requested clip exists with a plausible duration, or the error is in front of the user.
