#!/bin/bash
# Notification hook - plays audio when Claude needs user input
# Hook event: Notification

cat > /dev/null  # Consume stdin

SCRIPT_DIR="$(dirname "$0")"
AUDIO_FILE="$SCRIPT_DIR/utils/audio/needs_input.mp3"

if [[ -f "$AUDIO_FILE" ]]; then
    afplay "$AUDIO_FILE" &
fi

echo "{}"
