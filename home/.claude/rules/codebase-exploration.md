# Codebase Exploration

Route exploration through **scoutmaster**: one request in, one report of exact `path:line` findings out. It runs its own fan-out of scouts, so the searching spends its context instead of mine.

Hand it anything that spans multiple files, subsystems, or naming conventions -- "where does X happen", "map every caller of Y", "what touches Z", any hunt where the location is unknown. It is the first move for these, ahead of the built-in Explore agent and ahead of sweeping the repo myself.

Search directly when I already know the file, symbol, or value. A single lookup beats a dispatch.

Scoutmaster is the only caller of scouts; my requests go to scoutmaster.

Its report gives coordinates, not conclusions. Read the spans it names and reason over the code myself.
