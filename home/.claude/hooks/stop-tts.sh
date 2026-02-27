#!/bin/bash
# Stop hook - plays audio when Claude completes work
# Hook event: Stop

cat > /dev/null  # Consume stdin

SCRIPT_DIR="$(dirname "$0")"
AUDIO_FILE="$SCRIPT_DIR/utils/audio/work_complete.mp3"

if [[ -f "$AUDIO_FILE" ]]; then
    afplay "$AUDIO_FILE" &
fi

echo "{}"
