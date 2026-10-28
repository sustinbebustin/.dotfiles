#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Convert text to an MP3 with the ElevenLabs text-to-speech API.

Settings come from the skill's .env (ELEVENLABS_API_KEY, ELEVENLABS_VOICE_ID,
optional ELEVENLABS_MODEL_ID); a variable already set in the environment wins
over the file, and --model wins over both.
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

ENV_FILE = Path(__file__).resolve().parent.parent / ".env"
API_URL = "https://api.elevenlabs.io/v1/text-to-speech/{voice_id}?output_format=mp3_44100_128"
DEFAULT_MODEL = "eleven_v4"


def fail(message):
    print(f"error: {message}", file=sys.stderr)
    sys.exit(1)


def read_env_file(path):
    if not path.exists():
        return {}
    values = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        values[key.strip()] = value.strip().strip("\"'")
    return values


def lookup(name, file_values):
    return os.environ.get(name) or file_values.get(name)


def require(name, file_values):
    value = lookup(name, file_values)
    if not value:
        fail(f"{name} is not set. Copy {ENV_FILE.with_name('.env.example')} to {ENV_FILE} and fill it in.")
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--text", help="text to speak")
    source.add_argument("--text-file", type=Path, help="file whose contents to speak")
    parser.add_argument("--out", type=Path, required=True, help="output .mp3 path")
    parser.add_argument("--model", help=f"model_id (default: ELEVENLABS_MODEL_ID, else {DEFAULT_MODEL})")
    args = parser.parse_args()

    text = args.text if args.text is not None else args.text_file.read_text()
    if not text.strip():
        fail("text is empty; nothing to synthesize.")

    file_values = read_env_file(ENV_FILE)
    api_key = require("ELEVENLABS_API_KEY", file_values)
    voice_id = require("ELEVENLABS_VOICE_ID", file_values)
    args.model = args.model or lookup("ELEVENLABS_MODEL_ID", file_values) or DEFAULT_MODEL

    request = urllib.request.Request(
        API_URL.format(voice_id=voice_id),
        data=json.dumps({"text": text, "model_id": args.model}).encode(),
        headers={"xi-api-key": api_key, "Content-Type": "application/json", "Accept": "audio/mpeg"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request) as response:
            audio = response.read()
    except urllib.error.HTTPError as err:
        detail = err.read().decode(errors="replace")
        fail(f"ElevenLabs returned HTTP {err.code} for voice {voice_id}, model {args.model}: {detail}\nNo file was written.")
    except urllib.error.URLError as err:
        fail(f"could not reach api.elevenlabs.io ({err.reason}). Check the network and retry; no file was written.")

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_bytes(audio)
    print(f"{args.out} ({len(audio)} bytes, {len(text)} chars, model {args.model})")


if __name__ == "__main__":
    main()
