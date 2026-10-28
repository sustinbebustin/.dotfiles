---
name: elevenlabs-tts
description: Generate an MP3 from text with ElevenLabs text-to-speech, using the voice in the skill's .env.
argument-hint: "<text or path to a text file> [output.mp3]"
disable-model-invocation: true
allowed-tools: Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py *)
metadata:
  author: sustinbebustin
---

# ElevenLabs TTS

Turn `$ARGUMENTS` into one MP3.

## Steps

1. **Resolve the input.** If the first argument is an existing file path, speak that file; otherwise the argument text is the script. Output path is the second argument, else `./speech.mp3`; pick a non-clobbering name if that file exists.
2. **Check length.** Per-request character limits: 5,000 for `eleven_v3`, 40,000 for `eleven_flash_v2_5`, 10,000 for `eleven_v4` (the default) and most others. Over the limit, ask the user to shorten or split rather than truncating.
3. **Run:**

   ```bash
   uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --text "<text>" --out <path>
   # or, for a file or text with tricky quoting:
   uv run ${CLAUDE_SKILL_DIR}/scripts/tts.py --text-file <file> --out <path>
   ```

   The model comes from `ELEVENLABS_MODEL_ID` in `.env`; pass `--model <model_id>` only when the user names one for this run.
4. **Report** the path the script prints. On error, relay its message verbatim; a missing key means the user must create `${CLAUDE_SKILL_DIR}/.env` from `.env.example`.

Done when the MP3 exists at the reported path, or the script's error is in front of the user.
