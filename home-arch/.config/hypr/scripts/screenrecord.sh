#!/bin/bash
SAVE_DIR="$HOME/Videos/Recordings"
mkdir -p "$SAVE_DIR"

if pgrep -x wl-screenrec > /dev/null; then
    pkill -x wl-screenrec
    notify-send "Recording stopped" "Saved to $SAVE_DIR"
else
    FILENAME="$SAVE_DIR/recording-$(date +%Y%m%d-%H%M%S).mp4"
    wl-screenrec -f "$FILENAME" &
    notify-send "Recording started"
fi
