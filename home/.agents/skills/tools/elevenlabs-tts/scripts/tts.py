#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Convert text to MP3 with the ElevenLabs text-to-speech API.

Two modes:
  --text/--text-file + --out    one clip
  --manifest narration.json     a narration: one clip per entry, written next to
                                the manifest as NN-<id>.mp3, each request stitched
                                to the clips before it so prosody carries across

Manifest shape: {"clips": [{"id": "overview", "title": "Overview", "script": "..."}]}
Other keys on a clip are ignored.

Settings come from the skill's .env (ELEVENLABS_API_KEY, ELEVENLABS_VOICE_ID,
optional ELEVENLABS_MODEL_ID); a variable already set in the environment wins
over the file, and --model wins over both.
"""

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

ENV_FILE = Path(__file__).resolve().parent.parent / ".env"
API_URL = "https://api.elevenlabs.io/v1/text-to-speech/{voice_id}?output_format=mp3_44100_128"
BITS_PER_SECOND = 128_000
DEFAULT_MODEL = "eleven_v4"
# The API accepts at most 3 previous_request_ids.
STITCH_WINDOW = 3
CLIP_ID = re.compile(r"^[a-z0-9][a-z0-9-]*$")


@dataclass(frozen=True)
class Config:
    api_key: str
    voice_id: str
    model: str


@dataclass(frozen=True)
class Clip:
    id: str
    title: str
    script: str


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


def load_config(model_flag):
    file_values = read_env_file(ENV_FILE)
    return Config(
        api_key=require("ELEVENLABS_API_KEY", file_values),
        voice_id=require("ELEVENLABS_VOICE_ID", file_values),
        model=model_flag or lookup("ELEVENLABS_MODEL_ID", file_values) or DEFAULT_MODEL,
    )


def parse_manifest(path):
    try:
        raw = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as err:
        fail(f"could not read manifest {path}: {err}")
    entries = raw.get("clips") if isinstance(raw, dict) else None
    if not isinstance(entries, list) or not entries:
        fail(f'manifest {path} needs a non-empty "clips" list: {{"clips": [{{"id", "title", "script"}}]}}')
    clips = []
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict) or not all(isinstance(entry.get(k), str) and entry[k].strip() for k in ("id", "title", "script")):
            fail(f'manifest {path}, clip {index}: needs non-empty string "id", "title", and "script".')
        if not CLIP_ID.match(entry["id"]):
            fail(f'manifest {path}, clip {index}: id "{entry["id"]}" must be lowercase letters, digits, and hyphens.')
        clips.append(Clip(entry["id"], entry["title"], entry["script"]))
    duplicates = {c.id for c in clips if [d.id for d in clips].count(c.id) > 1}
    if duplicates:
        fail(f"manifest {path}: duplicate clip ids {sorted(duplicates)}.")
    return clips


def synthesize(config, text, previous_request_ids, next_text):
    """Return (mp3 bytes, request id) or exit with the API's error."""
    body = {"text": text, "model_id": config.model}
    if previous_request_ids:
        body["previous_request_ids"] = previous_request_ids
    if next_text:
        body["next_text"] = next_text
    request = urllib.request.Request(
        API_URL.format(voice_id=config.voice_id),
        data=json.dumps(body).encode(),
        headers={"xi-api-key": config.api_key, "Content-Type": "application/json", "Accept": "audio/mpeg"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request) as response:
            return response.read(), response.headers.get("request-id")
    except urllib.error.HTTPError as err:
        detail = err.read().decode(errors="replace")
        fail(f"ElevenLabs returned HTTP {err.code} for voice {config.voice_id}, model {config.model}: {detail}")
    except urllib.error.URLError as err:
        fail(f"could not reach api.elevenlabs.io ({err.reason}). Check the network and retry.")


def report(path, audio, text, model):
    seconds = len(audio) * 8 / BITS_PER_SECOND
    print(f"{path}  {seconds:.0f}s  {len(text)} chars  model {model}")


def run_single(config, text, out):
    if not text.strip():
        fail("text is empty; nothing to synthesize.")
    audio, _ = synthesize(config, text, [], None)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(audio)
    report(out, audio, text, config.model)


def run_manifest(config, manifest, only):
    clips = parse_manifest(manifest)
    unknown = only - {c.id for c in clips}
    if unknown:
        fail(f"--only names clips not in {manifest}: {sorted(unknown)}.")
    request_ids = []
    for index, clip in enumerate(clips):
        if only and clip.id not in only:
            continue
        following = clips[index + 1].script if index + 1 < len(clips) else None
        out = manifest.parent / f"{index + 1:02d}-{clip.id}.mp3"
        print(f"[{index + 1}/{len(clips)}] {clip.id}", file=sys.stderr)
        audio, request_id = synthesize(config, clip.script, request_ids[-STITCH_WINDOW:], following)
        out.write_bytes(audio)
        report(out, audio, clip.script, config.model)
        if request_id:
            request_ids.append(request_id)
        elif following:
            print(f"warning: no request-id returned for {clip.id}; the next clip is generated without stitching to it.", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--text", help="text to speak")
    source.add_argument("--text-file", type=Path, help="file whose contents to speak")
    source.add_argument("--manifest", type=Path, help="narration manifest JSON; clips are written beside it")
    parser.add_argument("--out", type=Path, help="output .mp3 path (single-clip mode)")
    parser.add_argument("--only", help="comma-separated clip ids to regenerate (manifest mode)")
    parser.add_argument("--model", help=f"model_id (default: ELEVENLABS_MODEL_ID, else {DEFAULT_MODEL})")
    args = parser.parse_args()

    if args.manifest:
        if args.out:
            parser.error("--out applies to single-clip mode; manifest clips are written beside the manifest")
        only = {s.strip() for s in args.only.split(",") if s.strip()} if args.only else set()
        run_manifest(load_config(args.model), args.manifest, only)
        return

    if not args.out:
        parser.error("--out is required with --text or --text-file")
    if args.only:
        parser.error("--only applies to --manifest mode")
    text = args.text if args.text is not None else args.text_file.read_text()
    run_single(load_config(args.model), text, args.out)


if __name__ == "__main__":
    main()
