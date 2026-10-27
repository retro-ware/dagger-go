# Deviations from FALL.EXE

The port follows the lifted C (dagger-re/src) and the engine notes (dagger-re/docs). This
file lists where it does something else, and why. Small guards are marked in the code with
`port:` comments (`grep -rn "port:" internal`); the ones below are grouped by kind.

## Hardware and DOS: replaced by host code

- DPMI and CauseWay memory (int.c), jmem, the profiler, the VR helmet, the serial link and
  the PC/system interrupt vectors have no port. The host (internal/host: gogpu + WebGPU
  window, oto audio) takes their place.
- The SOS digital driver is internal/xn/sound.go, mixed by internal/host/audio.go. A
  sample's done callback runs from Poll on the game's goroutine, and the mixer stops a chunk
  where a chained sample ends, so chained buffers still follow without a gap.
- Music: the DOS MPU-401/OPL drivers are replaced by a General MIDI synth (meltysynth, MIT)
  and a SoundFont read at start from the working directory: the first *.sf2 there (e.g.
  SC55_zzdenis_v0.5.sf2, or GeneralUser GS from tools/fetch-soundfont.sh), unless
  `$DAGGER_SF2` names a file. The HMI
  parser (internal/music/hmi.go) reads MIDI.BSA's GM songs; the SOS controllers 102-119
  are dropped, and a note of length 0 (37 in the songs) sounds for a tick. With no SoundFont there is no music, as with no MIDI device. A track end far
  past a song's last note is ignored: SUNNYDAY.HMI's track 5 sets the reverb 2.8 hours
  after its music ends, so under DOS the song only came round again after that silence.
- The mouse: the original polled the DOS int 33h driver. The port uses a modern pointer
  (internal/xn/pointer.go): the game's cursor follows the OS pointer (hidden over the
  picture) directly, clamped to the game's ranges, and the game placing its cursor moves
  the OS pointer; mouse look, the free look and weapon swings lock the pointer and read its
  raw motion (the game asks each frame, so menus get the pointer back by themselves; losing
  the focus releases it). Motion is the same across and down (the DOS driver's default
  doubled it down) and doesn't depend on the window size: a point of locked motion is a
  mickey, a step of the view (0.18 degrees, about a pixel of the host's mouse under
  DOSBox; Daggerfall Unity's default is 0.2 degrees), times the HORIZONTAL and VERTICAL
  settings (GOG's BETAPLYR.DAT has 5: 0.6). When a screen over the world closes (its own
  loop, or the world's frames with GameMode or ScreenActive set: uiResume), the keys still
  held from it don't act again (the Enter that ends a save's name turned the cursor on)
  and the mouse's motion over it doesn't turn the view. gogpu neither moves nor locks the
  pointer on macOS, so gpu/cursor_darwin.go does both through CoreGraphics (a lock holds
  the hidden pointer at the window's centre and reads its moves); elsewhere a move the
  window can't make offsets the cursor from the OS pointer.
- Video memory: the window gets a frame when the game shows video memory, but a palette at
  once. On the VGA both are live, so writing the palette hands over video memory as it is
  (xn.dacFlush): a palette set after the game cleared the screen no longer recolours the
  frame shown before (the title in the province map's palette, between the two).
- The window scales the frame at its own aspect ratio, with black bars beside or above it.
- No joystick: the joystick screen and the joystick's bindings are cut. The CONTROLS
  screen's JOYSTICK button is greyed and does nothing, a joystick binding in an old controls
  file never fires, and the file's joystick bytes are written as 0.
- Frames: the DOS game ran as fast as the PC allowed and measured each frame's time in its
  140 Hz timer's ticks. The port paces frames to the display (internal/host/pace.go): a game
  frame each refresh, or each n-th refresh on a fast display, at most 75 a second, so each
  stays on screen equally long; a frame's ticks come from the clock with the fraction
  carried (FrameTicksUpdate). Some steps still count frames (guards' arrival, fatigue,
  message delays), so the game rate stays near the 70 a second the port was tested at.
- Moves: a frame's move keeps its fraction of a unit for the next frame (carryWhole), as
  FALL.EXE's pedestrians keep theirs. FALL.EXE moved the player and the creatures whole
  units a frame, rounded down, which at 60 frames a second cost a quarter of the walk, half
  of a slide, half a jump's height and up to a fifth of a creature's pace (at 23, its first
  guess of its frame rate, far less). The player counts as moving (the head bob, the bonus to
  hit) above 46 units a second, FALL.EXE's 2 a frame at 23 frames a second.
- A screen with a loop of its own (the save menu, a message box) hands the window a frame
  that changes nothing only once the window has shown one since (host presentPace): such
  loops went round 190,000 times a second and kept a core busy.

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

All off by default but the WASD keys and the diagonal moves; the Enhancements screen (a
line under Exit on the title menu, a button under the options' panel) turns them on and
off, and ENHANCE.CFG next to the saves keeps them (internal/game/enhance.go):

- Mouse look: the CONTROLS screen's VIEW mode, and the mouse looks around in the air too.
  Enter (ACTIVATE CURSOR) still brings back a cursor for the HUD, until a screen over the
  world closes. The mouse turns the view before the frame is drawn, not after: the turn
  shows a frame sooner.
- Activate: the left button uses what the crosshair is on (as ACTIVATE CENTER OBJECT).
- Weapon swing: Drag (the original's), Click (each press of SWING WEAPON swings once) or
  Hold (it swings while held); Click and Hold pick a random swing, as Daggerfall Unity, and
  the view keeps turning during them. The bow is unchanged.
- Keys: the WASD preset, on by default: Daggerfall Unity's defaults. W/S/A/D move, Space
  jumps, C crouches, Ctrl slides, Shift runs, V the travel map, Z readies the weapon, H
  switches hands; the rest keep their keys. Daggerfall Unity uses with the left button and
  aborts a spell with E; here E uses (the left button is the Activate enhancement, as a
  binding would fire at a click with the HUD's cursor too) and G aborts. At start it
  replaces keys still at the defaults only, and the CONTROLS screen's DEFAULT gives the
  preset's. "Original" gives back FALL.EXE's keys (the arrows, the Shifts, J, D, P...).
- Movement: Diagonal, on by default: a slide (SLIDE LEFT or RIGHT, or a turn with SLIDE)
  with FORWARD or REVERSE moves both ways at once, each at 0.7071 of its own speed, as
  Daggerfall Unity and Daggerfall Online move; FALL.EXE only slides. Forward is FALL.EXE's
  255/220 of the walk and a slide 255/256, so forward and sideways is 41 degrees off
  ahead at 1.08 of the walk, back and sideways 45 degrees at 1.0. "Original" slides.
- Crosshair: a small plus at the view's centre while the mouse looks around.
- Hardware 3D: the GPU draws the 3D view at the window's resolution (internal/xn/hw3d.go,
  internal/host/r3d.go). The engine still culls, lights and sorts as FALL.EXE does; the
  models, the ground and the flats go to the GPU as triangles, which it transforms, clips
  and rasterises with the game's own 8-bit textures, shade, fog and translucency tables,
  so the colours stay the palette's. One pass draws it; its colour indices, what each
  pixel shows and its depth are read back, and picking and the back buffer take the
  pixel at each screen pixel's centre. The window shows the sharp view wherever the frame still
  shows the 3D view. The dungeon water is drawn over it at the window's resolution by the
  composite, with the original's ripple (per-row jitter, phase and tint, in screen pixels);
  the 320 x 200 screen is tinted too, as the original does, for the game's own uses. Only the textured mode at 320 x 200 uses it; when the GPU fails the
  game falls back to the software renderer. While the mouse looks around, the view turns
  on by the fraction of a step the mouse's motion left (a step is 5 or more of the
  window's pixels, so a slow turn moved in jumps), and is drawn about 1% larger, so the
  edge a turn brings in is never one the engine culled.

## Testing aids (not in the original)

`-skipintro`, `-quickstart`, `-load N`, `-town`, `-quest`, `-talk`, `-screen` and the
`-input`/`-shot` script mode (cmd/dagger).
