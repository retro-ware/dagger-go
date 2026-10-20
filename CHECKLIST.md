# Checklist

## Prep (dagger-re)
- [ ] src/readable fork, built by the native harness
- [ ] deterministic virtual-time harness: native(src) and native(readable) match
- [ ] offsets typed, punning removed
- [ ] disk formats split from runtime structs

## Go
- [x] module skeleton, plan
- [x] window + framebuffer presenter (gpu)
- [x] audio (oto): 8-bit voices mixed to 44.1 kHz stereo; chained samples on one handle
- [x] engine (internal/xn): every XnGine unit except the DOS hardware layer (helmet, joy
      hardware, serial, pc/sys DPMI and vectors, mem locks); tables from FALL.EXE object 2
- [x] spell thunks (with the game's spell dispatch)
- [x] DEVIATIONS.md
- [x] game: every module of FALL.EXE's game code, including spells, monsters/AI, combat,
      automap and town map, talk, quests, factions, guild makers (spell, potion, item,
      class), music (HMI through a GM synth); the dead notebook (note.c) is not ported
- [x] format readers (BSA, IMG/CIF/RCI, CFA, TEXT.RSC, QBN/QRC, HMI, VID, saves)
- [ ] parity traces
