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
  and the GeneralUser GS SoundFont (tools/fetch-soundfont.sh, or `$DAGGER_SF2`). The HMI
  parser (internal/music/hmi.go) reads MIDI.BSA's GM songs; the SOS controllers 102-119
  are dropped. With no SoundFont there is no music, as with no MIDI device. Faithful quirk:
  SUNNYDAY.HMI has a track with a giant delta, so the song only loops after ~2.8 hours.
- The mouse is a model of the int 33h driver (internal/xn/mousedrv.go) fed by host events.
- No joystick is read: joystick_status stays 2 (none found), so the joystick screens show
  a centred stick.
- Frames: the DOS game never outran its 140 Hz timer; the port waits for two of its ticks
  per frame (internal/game/main.go).

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
- The class maker opens a class file only when it exists.
- The save menu's thumbnails are taken as the menu opens (the original kept one in
  big_buffer), and BigBuffer keeps the screen under a paged message box.

## Kept as the original does (faithful, though odd)

- spellCostItemPercent is 100: the original means 75% under an item's spell-point
  enchantment but reads the character record as the item.
- potionCauldronCount is never raised, as in the original.
- The class maker's "clear magery" resets all the flags to 5120.
- AutomapPurgeOldFiles deletes nothing (the original passes the search pattern to unlink).
- The quest debug overlay shows the stale text_buffer, and the side-effect exclusion uses
  scratch_190d66, as the original.

## Dead code not ported

- note.c, the notebook: note_update has no callers (docs/state.md).
- talk_translate and talk_garble_word: never reached.
- potion_have_ingredients: no callers.

## Testing aids (not in the original)

`-skipintro`, `-quickstart`, `-load N`, `-town`, `-quest`, `-talk`, `-screen` and the
`-input`/`-shot` script mode (cmd/dagger).
