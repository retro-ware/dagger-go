# Daggerfall in Go: porting plan

A native desktop port of The Elder Scrolls II: Daggerfall (FALL.EXE), written in
idiomatic Go from the readable C decompilation in ../dagger-re. This is a standalone
desktop app, not a goplay engine. It does not emulate a CPU and it does not read exe
tables at runtime. The player points it at an installed Daggerfall directory with
`-data DIR`.

## Stack (as in ../por-go)

- `gpu` (gogpu + WebGPU, pure Go: `CGO_ENABLED=0 go build -tags nofakecgo ./cmd/dagger`),
  the module in ./gpu, for the window, input and presenting the 320x200 framebuffer.
- `-import SRC` finds the game (folders, zips, CD images, GOG installers, loose files,
  through ../gameimport) and copies FALL.EXE, Z.CFG and ARENA2 into -data. Run with no
  flags in a goplay folder, it reads ./data, writes ./save and plays the SoundFont in
  ../../system/sf2.
- `github.com/ebitengine/oto/v3` for audio.
- The engine runs on its own goroutine and writes into a framebuffer. Presenters show it.

## Source of truth

- `dagger-re/src/engine/` holds the readable XnGine C. `src/lifted`, `src/hand` and
  `src/*.c` hold the game C.
- `dagger-re/port/` is the native C port (SDL3), the behavioural reference.
- `dagger-re/tools/port_compare.py` and `fallplay.py` run lockstep comparisons.

## References

- https://github.com/Lattymoy/daggerfall-js-source is a JavaScript Daggerfall source, used
  as a cross-check. dagger-re's C stays the source of truth.

## Phases

1. Prep, in dagger-re:
   - Add src/readable, a fork with no Watcom matching constraints.
   - Add a deterministic virtual-time harness.
   - Type the offsets and remove the punning.
   - Split the on-disk formats from the runtime structs.
2. Formats: Go readers for the data files (BSA, CIF/IMG/TEXTURE, RCI, palettes, maps,
   WOODS/CLIMATE, sound), each tested against the C reader's dumps.
3. Platform layer: the window, framebuffer, palette, input, timer and audio.
4. Engine: the XnGine renderer, translated subsystem by subsystem.
5. Game: the UI, character creation, world, quests and combat.
6. Parity: lockstep traces against the native C port.

## Rules

- Every value is a typed variable. No raw memory blocks or offsets.
- Native desktop only: no Electron and no web UI shells.
- After each milestone, update CHECKLIST.md and commit.
