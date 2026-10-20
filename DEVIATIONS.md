# Deviations from FALL.EXE

The port follows the lifted C (dagger-re/src) and the engine notes (dagger-re/docs). This
file lists where it does something else, and why. Small guards are marked in the code with
`port:` comments (`grep -rn "port:" internal`); the ones below are grouped by kind.

## Hardware and DOS: replaced by host code

- DPMI and CauseWay memory (int.c), jmem, the profiler, the VR helmet, the serial link and
  the PC/system interrupt vectors have no port. The host (internal/host: GLFW + WebGPU
  window, oto audio) takes their place.
- The SOS digital driver is internal/xn/sound.go, mixed by internal/host/audio.go. A
  sample's done callback runs from Poll on the game's goroutine, and the mixer stops a chunk
  where a chained sample ends, so chained buffers still follow without a gap.
- Music: the DOS MPU-401/OPL drivers are replaced by a General MIDI synth (meltysynth, MIT)
  and a SoundFont read at start from the working directory: the first *.sf2 there (e.g.
  SC55_zzdenis_v0.5.sf2, or GeneralUser GS from tools/fetch-soundfont.sh), unless
  `$DAGGER_SF2` names a file. The HMI
  parser (internal/music/hmi.go) reads MIDI.BSA's GM songs; the SOS controllers 102-119
  are dropped. With no SoundFont there is no music, as with no MIDI device. A track end far
  past a song's last note is ignored: SUNNYDAY.HMI's track 5 sets the reverb 2.8 hours
  after its music ends, so under DOS the song only came round again after that silence.
- The mouse: the original polled the DOS int 33h driver. The port uses a modern pointer
  (internal/xn/pointer.go): the game's cursor follows the OS pointer (hidden over the
  window) directly, clamped to the game's ranges; mouse look, the free look and weapon
  swings lock the pointer and read its raw motion (the game asks each frame, so menus get
  the pointer back by themselves; losing the focus releases it). Motion is the same across
  and down (the DOS driver's default doubled it down) and doesn't depend on the window size.
- No joystick is read: joystick_status stays 2 (none found), so the joystick screens show
  a centred stick.
- Frames: the DOS game ran as fast as the PC allowed and measured each frame's time in its
  140 Hz timer's ticks. The port paces frames to the display (internal/host/pace.go): a game
  frame each refresh, or each n-th refresh on a fast display, at most 75 a second, so each
  stays on screen equally long; a frame's ticks come from the clock with the fraction
  carried (FrameTicksUpdate). Some steps still count frames (guards' arrival, fatigue,
  message delays), so the game rate stays near the 70 a second the port was tested at.

## Engine quirks dropped (see dagger-re/docs/engine/quirks.md)

- Q-LIGHT-04: the squares table index is unbounded; out of range the port reads 0.
- Q-LIGHT-07: the compiled light shaders' bytes in big_buffer are not reproduced; the
  shaders are records only.
- Q-ANIM-01/02/03/05: the dead reset_with_rate store, restart's low-memory position (here
  the script stops), the 4 GB position wrap, and opcodes from 12 up (stop).
- Q-VID-03: an unopened movie ends instead of reading handle 0. Q-VID-04: a raw frame
  crossing a buffer refill keeps its width. D-VID-01: the region locks are gone.
- The texture heap is read at offset 0 and the texture-mapper pool is kept as a count, not
  as DOS memory.
- The world generator's river and path passes are disabled in the original and not ported.

## Null and stale pointers: guarded

Where the original read or wrote through a null, stale or out-of-range pointer (reading the
DOS zero page, writing past a buffer), the port returns or uses 0 instead. Among them:
automap notes overflow and the town notes buffer, the town map's missing building and
out-of-map pixels, the loot pile of a missing building, the inventory's null container,
quest state writes through a null state, a missing faction or building in dialogue, a monster
poisoning a null weapon or casting a null spell, the item maker consuming a soul through a
stale pointer, and the save's faction links (memory pointers, 0 on load).
The full list is the `port:` comments.

## Small fixes where the C read garbage

- Inventory: the weight text is drawn in the cell's own rectangle (C indexed it by a loop's
  leftover counter); a running total starts at 0.
- Travel: the C loaded the location past the last one; the last one is loaded.
- Quests: a retry counter the C never counted up is counted, so the search ends.
- Spell costs: an equipped item's spell-point enchantment cuts a spell's cost to 75% while
  its condition holds (spellCostItemPercent). The original read the character record's slot
  array as the item, so the discount never applied, and its creature check never stepped.
- The class maker opens a class file only when it exists.
- The save menu's thumbnails are taken as the menu opens (the original kept one in
  big_buffer), and BigBuffer keeps the screen under a paged message box.

## Kept as the original does (faithful, though odd)

- potionCauldronCount is never raised, as in the original.
- The class maker's "clear magery" resets all the flags to 5120.
- The automap's old-files purge (delete notes and maps unvisited for 90 days) is dropped:
  the original passed the search pattern to unlink, so it never deleted anything.
- The quest debug overlay shows the stale text_buffer, and the side-effect exclusion uses
  scratch_190d66, as the original.

## Dead code not ported

- note.c, the notebook: note_update has no callers (docs/state.md).
- talk_translate and talk_garble_word: never reached.
- potion_have_ingredients: no callers.

## Enhancements (opt-in, after Daggerfall Unity)

All off by default; the Enhancements screen (a line under Exit on the title menu, a button
under the options' panel) turns them on, and ENHANCE.CFG next to the saves keeps them
(internal/game/enhance.go):

- Mouse look: the CONTROLS screen's VIEW mode, and the mouse looks around in the air too.
  Enter (ACTIVATE CURSOR) still brings back a cursor for the HUD.
- Activate: the left button uses what the crosshair is on (as ACTIVATE CENTER OBJECT).
- Weapon swing: Drag (the original's), Click (each press of SWING WEAPON swings once) or
  Hold (it swings while held); Click and Hold pick a random swing, as Daggerfall Unity, and
  the view keeps turning during them. The bow is unchanged.
- Keys: the WASD preset. W/S/A/D move, Space jumps, C crouches, Ctrl slides, Shift runs, E
  uses, V the travel map, G aborts a spell, Z readies the weapon, H switches hands; the rest
  keep their keys. At start it replaces keys still at the defaults only.
- Crosshair: a small plus at the view's centre while the mouse looks around.
- Hardware 3D: the GPU draws the 3D view at the window's resolution (internal/xn/hw3d.go,
  internal/host/r3d.go). The engine still culls, lights and sorts as FALL.EXE does; the
  models, the ground and the flats go to the GPU as triangles, which it transforms, clips
  and rasterises with the game's own 8-bit textures, shade, fog and translucency tables,
  so the colours stay the palette's. A 320 x 200 pass of the same scene feeds picking,
  water and the back buffer, and the window shows the sharp view wherever the frame still
  shows the 3D view. Only the textured mode at 320 x 200 uses it; when the GPU fails the
  game falls back to the software renderer.

## Testing aids (not in the original)

`-skipintro`, `-quickstart`, `-load N`, `-town`, `-quest`, `-talk`, `-screen` and the
`-input`/`-shot` script mode (cmd/dagger).
