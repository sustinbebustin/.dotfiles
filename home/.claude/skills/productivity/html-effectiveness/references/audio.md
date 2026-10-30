# Audio narration

Spoken explanation layered on a page, for a reader who learns by listening. Add it only on request: audio costs real money per character and goes stale when the page changes.

The audio is a guide talking the reader through the page they are looking at. It does what the page cannot: walks a diagram in the order to look at it, says why a thing matters, joins the sections into one story. It never reads the page aloud.

## Plan the clips

| Request | Clips |
|---------|-------|
| "an audio explanation", "a summary I can listen to" | One overview clip. Dock only. |
| "audio for the sections", "walk me through it" | An overview clip, then one clip per section whose understanding rides on a picture, a sequence, or a subtle distinction. Reference tables and footnotes get none. |
| A section with a view switcher (tabs, scenario picker) | One clip per view, each flipping the switcher to its view as it starts (`activate`). |

The overview runs 45-90 seconds: the question the page answers, the two or three things to walk away with, and how the page is laid out. Section clips run 30-120 seconds. The main path, overview plus section clips, lands near 6-10 minutes; past that, cut. Per-view clips come on top, since a listener picks the views they care about.

## Write the scripts

Invoke the `elevenlabs-tts` skill: its script rules (spoken numbers, symbols, identifiers, signposting, pacing) apply to every clip. On top of them, for a page:

- **Point at what is on screen.** The listener is looking at the section. Name things by what they see -- "the grey column on the left", "the red dotted line", "dot three on the timeline" -- then say what it means. Walk a diagram in the order the eye should travel.
- **Interpret, not transcribe.** Labels and captions are already on screen. The clip says why the arrow is there, what would surprise a newcomer, what the rule is underneath the example.
- **Connect.** Open a section clip by tying it to the last ("That thirty-day clock is what the rep's page checks first."). Close it on the one sentence to remember.
- **Same names as the page.** Spoken terms match the headings and labels so eyes can find what the ear heard; the spoken form of an identifier ("selection unavailable") is still recognisable next to the written one.
- **Every fact from the page.** No number, rule, or outcome the page does not state.

## Generate

Write the manifest beside the page as `<page-stem>-audio/narration.json`, ids matching section ids, then generate with the `elevenlabs-tts` skill's narration mode. Give each clip the player's page fields too -- `section` and, for a view, `activate` (see the JS below); generation ignores them, so the manifest stays the one source for both audio and player. Report the total character count before generating a set over 5,000 characters. Clips land as `<page-stem>-audio/NN-<id>.mp3`.

The page references clips by relative path, so it plays straight from disk with the folder beside it. Publishing it as an Artifact means passing each mp3 in `files`. Inlining as base64 makes a single file 1.33x the audio size and too large for an agent to read back; do it only when the user asks for one self-contained file.

## The player

A dock fixed to the bottom of the viewport plays the clips as a playlist: play all from the start, or start at any section with its **Listen** button; each clip runs on into the next. While a clip plays, its section carries a clay rule, its Listen button shows as pressed, and auto-advance scrolls the next section into view. Speed (1x to 2x) persists per viewer. A transcript panel shows the current clip's script. Lock-screen and headphone controls work through the Media Session API. Nothing autoplays.

Copy the three blocks below verbatim, then fill `NARRATION` and place one Listen button under each narrated section's heading. Overview-only pages skip the Listen buttons. A section with a view switcher gets one Listen button whose `data-clip` the page's switch handler retargets to the selected view's clip.

Build `NARRATION` from the manifest rather than retyping the scripts (replace `STEM`):

```bash
jq '[.clips | to_entries[] | (.key + 1) as $n | .value + {src: ("STEM-audio/" + (if $n < 10 then "0" else "" end) + ($n | tostring) + "-" + .value.id + ".mp3")}]' STEM-audio/narration.json
```

### Markup

```html
<!-- Under a narrated section's h2. data-clip = the clip id; .dur = the clip length from the generate output. -->
<button class="listen" type="button" data-clip="the-clock" aria-pressed="false">
  <span class="ic" aria-hidden="true"><svg viewBox="0 0 10 10"><path d="M2 1 L9 5 L2 9 z"/></svg></span>Listen <span class="dur">1:12</span>
</button>

<!-- Last element in <body>. -->
<div class="narration" role="region" aria-label="Audio narration">
  <div class="np-transcript" id="np-transcript" hidden></div>
  <div class="np-ctl">
    <button class="np-btn np-prev" type="button" aria-label="Previous clip"><svg viewBox="0 0 12 12"><path d="M3 2v8M10 2 L4 6 L10 10z"/></svg></button>
    <button class="np-btn np-play" type="button" aria-label="Play narration"><svg viewBox="0 0 12 12"><path class="i-play" d="M3 1.5 L10.5 6 L3 10.5z"/><path class="i-pause" d="M3 1.5h2.2v9H3zM6.8 1.5H9v9H6.8z"/></svg></button>
    <button class="np-btn np-next" type="button" aria-label="Next clip"><svg viewBox="0 0 12 12"><path d="M9 2v8M2 2 L8 6 L2 10z"/></svg></button>
  </div>
  <div class="np-mid">
    <div class="np-meta"><span class="np-title">Listen to this page</span><span class="np-count"></span></div>
    <input class="np-seek" type="range" min="0" max="1000" value="0" aria-label="Seek within clip">
  </div>
  <div class="np-side">
    <span class="np-time">0:00</span>
    <button class="np-pill np-rate" type="button" aria-label="Playback speed">1x</button>
    <button class="np-pill np-tx" type="button" aria-expanded="false" aria-controls="np-transcript">Transcript</button>
  </div>
</div>
```

### CSS

```css
/* Narration: Listen buttons + docked player */
body.has-narration { padding-bottom: 140px; }
.is-narrating { position: relative; }
.is-narrating::before { content: ''; position: absolute; left: -14px; top: 2px; bottom: 0; width: 3px; border-radius: 2px; background: var(--clay); }
.listen { display: inline-flex; align-items: center; gap: 8px; font: 500 11.5px/1 var(--mono); letter-spacing: 0.04em; text-transform: uppercase; color: var(--slate); background: var(--white); border: 1.5px solid var(--gray-300); border-radius: 999px; padding: 4px 12px 4px 4px; margin: 0 0 14px; cursor: pointer; }
.listen:hover { border-color: var(--gray-500); }
.listen .ic { width: 22px; height: 22px; border-radius: 50%; background: var(--slate); display: inline-flex; align-items: center; justify-content: center; }
.listen .ic svg { width: 9px; height: 9px; fill: #fff; margin-left: 1px; }
.listen .dur { color: var(--gray-500); }
.listen[aria-pressed="true"] { border-color: var(--clay); }
.listen[aria-pressed="true"] .ic { background: var(--clay); }

.narration { position: fixed; z-index: 20; left: 50%; bottom: 16px; transform: translateX(-50%); width: min(760px, calc(100% - 32px)); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px 14px; background: var(--white); border: 1.5px solid var(--gray-300); border-radius: 12px; padding: 10px 14px; }
.np-ctl, .np-side { display: flex; align-items: center; gap: 6px; }
.np-btn { width: 30px; height: 30px; border-radius: 50%; border: 1.5px solid var(--gray-300); background: var(--white); display: inline-flex; align-items: center; justify-content: center; cursor: pointer; }
.np-btn svg { width: 12px; height: 12px; fill: var(--slate); stroke: var(--slate); stroke-width: 1.2; }
.np-btn:disabled { opacity: 0.35; cursor: default; }
.np-play { width: 38px; height: 38px; background: var(--slate); border-color: var(--slate); }
.np-play svg { fill: #fff; stroke: none; }
.np-play .i-pause, .narration.playing .np-play .i-play { display: none; }
.narration.playing .np-play .i-pause { display: inline; }
.np-meta { display: flex; justify-content: space-between; gap: 10px; font-size: 13px; color: var(--slate); margin-bottom: 4px; }
.np-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--serif); font-size: 14.5px; }
.np-title.err { color: var(--rust); font-family: var(--sans); font-size: 13px; }
.np-count { font-family: var(--mono); font-size: 11px; color: var(--gray-500); white-space: nowrap; }
.np-seek { width: 100%; accent-color: var(--clay); display: block; }
.np-time { font-family: var(--mono); font-size: 11.5px; color: var(--gray-500); font-variant-numeric: tabular-nums; min-width: 76px; text-align: right; }
.np-pill { font-family: var(--mono); font-size: 11px; padding: 5px 9px; border-radius: 8px; border: 1.5px solid var(--gray-300); background: var(--gray-150); color: var(--gray-700); cursor: pointer; }
.np-pill[aria-expanded="true"] { background: var(--oat); border-color: #CDBFA8; color: var(--slate); }
.np-transcript { grid-column: 1 / -1; max-height: 32vh; overflow-y: auto; font-size: 14px; color: var(--gray-700); border-bottom: 1.5px solid var(--gray-300); padding: 4px 2px 12px; }
.np-transcript p { max-width: 68ch; margin-bottom: 8px; }
@media (max-width: 600px) {
  body.has-narration { padding-bottom: 170px; }
  .is-narrating::before { left: -10px; }
  .narration { grid-template-columns: auto minmax(0, 1fr); bottom: 8px; width: calc(100% - 16px); }
  .np-side { grid-column: 1 / -1; justify-content: flex-end; }
}
```

### JS

```js
/* Narration player. One entry per clip, in play order.
   section:  the element the clip explains; highlighted while it plays.
   activate: optional control to click when the clip starts, so the view matches what is said.
   script:   the spoken text, shown in the transcript panel. */
const NARRATION = [
  { id: 'overview', title: 'Overview', src: 'page-audio/01-overview.mp3', section: 'header.page-head', script: '...' },
];

(() => {
  const dock = document.querySelector('.narration');
  if (!dock || !NARRATION.length) return;
  document.body.classList.add('has-narration');
  const $ = s => dock.querySelector(s);
  const playBtn = $('.np-play'), prevBtn = $('.np-prev'), nextBtn = $('.np-next'), seek = $('.np-seek');
  const title = $('.np-title'), count = $('.np-count'), time = $('.np-time'), rateBtn = $('.np-rate'), txBtn = $('.np-tx'), tx = $('.np-transcript');
  const listens = [...document.querySelectorAll('.listen[data-clip]')];
  const RATES = [1, 1.25, 1.5, 1.75, 2];
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const audio = new Audio();
  audio.preload = 'none';
  let cur = -1;

  const fmt = s => Number.isFinite(s) ? `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}` : '0:00';
  const clipEl = c => c.section ? document.querySelector(c.section) : null;

  let rate = 1;
  try { rate = RATES.includes(+localStorage.getItem('narration-rate')) ? +localStorage.getItem('narration-rate') : 1; } catch {}
  const applyRate = () => { audio.defaultPlaybackRate = audio.playbackRate = rate; rateBtn.textContent = `${rate}x`; };
  applyRate();

  function load(n) {
    cur = n;
    const c = NARRATION[n];
    audio.src = c.src;
    applyRate();
    title.textContent = c.title;
    title.classList.remove('err');
    count.textContent = NARRATION.length > 1 ? `${n + 1} of ${NARRATION.length}` : '';
    tx.innerHTML = '';
    for (const para of c.script.split(/\n\s*\n/)) tx.appendChild(document.createElement('p')).textContent = para;
    document.querySelectorAll('.is-narrating').forEach(el => el.classList.remove('is-narrating'));
    clipEl(c)?.classList.add('is-narrating');
    if (c.activate) { const el = document.querySelector(c.activate); if (el && !el.checked) el.click(); }
    if ('mediaSession' in navigator) navigator.mediaSession.metadata = new MediaMetadata({ title: c.title, artist: document.title });
    sync();
  }
  function reveal() {
    const el = clipEl(NARRATION[cur]);
    if (!el) return;
    const r = el.getBoundingClientRect();
    if (r.top < 0 || r.top > innerHeight * 0.6) el.scrollIntoView({ behavior: reduced ? 'auto' : 'smooth', block: 'start' });
  }
  function play(n, scroll) {
    if (n !== cur) load(n);
    if (scroll) reveal();
    audio.play().catch(() => {});
  }
  function toggle(n) { n === cur && !audio.paused ? audio.pause() : play(n, false); }
  function sync() {
    const playing = !audio.paused && !audio.ended;
    dock.classList.toggle('playing', playing);
    playBtn.setAttribute('aria-label', playing ? 'Pause narration' : 'Play narration');
    prevBtn.disabled = cur <= 0;
    nextBtn.disabled = cur < 0 || cur >= NARRATION.length - 1;
    for (const b of listens) b.setAttribute('aria-pressed', String(playing && NARRATION[cur]?.id === b.dataset.clip));
  }
  function tick() {
    time.textContent = `${fmt(audio.currentTime)} / ${fmt(audio.duration)}`;
    if (audio.duration) seek.value = String(Math.round(1000 * audio.currentTime / audio.duration));
  }

  playBtn.addEventListener('click', () => toggle(cur < 0 ? 0 : cur));
  prevBtn.addEventListener('click', () => cur > 0 && play(cur - 1, true));
  nextBtn.addEventListener('click', () => cur < NARRATION.length - 1 && play(cur + 1, true));
  seek.addEventListener('input', () => { if (audio.duration) audio.currentTime = audio.duration * seek.value / 1000; });
  rateBtn.addEventListener('click', () => {
    rate = RATES[(RATES.indexOf(rate) + 1) % RATES.length];
    applyRate();
    try { localStorage.setItem('narration-rate', String(rate)); } catch {}
  });
  txBtn.addEventListener('click', () => {
    tx.hidden = !tx.hidden;
    txBtn.setAttribute('aria-expanded', String(!tx.hidden));
    if (cur < 0) load(0);
  });
  // data-clip is read on click, so page code may retarget a button (e.g. to the selected view's clip).
  for (const b of listens) b.addEventListener('click', () => {
    const n = NARRATION.findIndex(c => c.id === b.dataset.clip);
    if (n >= 0) toggle(n);
  });
  ['play', 'pause', 'ended'].forEach(e => audio.addEventListener(e, sync));
  ['timeupdate', 'loadedmetadata', 'durationchange'].forEach(e => audio.addEventListener(e, tick));
  audio.addEventListener('ended', () => { if (cur < NARRATION.length - 1) play(cur + 1, true); });
  audio.addEventListener('error', () => {
    title.textContent = `Audio not found: ${NARRATION[cur].src} -- keep the audio folder beside this page.`;
    title.classList.add('err');
    sync();
  });
  if ('mediaSession' in navigator) {
    const on = (a, f) => { try { navigator.mediaSession.setActionHandler(a, f); } catch {} };
    on('play', () => toggle(cur < 0 ? 0 : cur));
    on('pause', () => audio.pause());
    on('previoustrack', () => cur > 0 && play(cur - 1, true));
    on('nexttrack', () => cur < NARRATION.length - 1 && play(cur + 1, true));
  }
  sync();
})();
```

## Verify

On top of the page checks in the main process:

- Every clip's duration from the generate output is plausible for its length (about 15 characters a second); regenerate outliers with `--only`.
- In the browser: no console errors; play from the dock and from one Listen button, and confirm `currentTime` advances, the section takes the clay rule, and the Listen button reads pressed; skip to the last clip and let it end; open the transcript.
- At ~390px the dock covers nothing it should not: scroll to the bottom and check the last section and footer clear it.
