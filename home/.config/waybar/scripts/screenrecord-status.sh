#!/bin/bash

# Check if wl-screenrec is running
if pgrep -x wl-screenrec > /dev/null; then
    # Recording is active
    echo "{\"text\":\"󰑊\", \"class\":\"recording\", \"tooltip\":\"Recording in progress\nClick to stop\"}"
else
    # Not recording
    echo "{\"text\":\"󰻃\", \"class\":\"idle\", \"tooltip\":\"Click to start recording\"}"
fi
