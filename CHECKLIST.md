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
- [ ] format readers
- [x] engine (internal/xn): every XnGine unit except the DOS hardware layer (helmet, joy
      hardware, serial, pc/sys DPMI and vectors, mem locks); tables from FALL.EXE object 2
- [ ] spell thunks (with the game's spell dispatch)
- [ ] DEVIATIONS.md (Q-LIGHT-04/07, tex heap read at 0, tmap pool as a count, Q-ANIM-01..03,
      Q-VID-03/04, D-VID-01, world river/path passes, hardware layer)
- [ ] game: done init, support, archive, object, args, config, disk, color, intro, kludge,
      book, intrface, keys, camera, rumor, picklist, people, steal, region, trade haggling,
      song, parse (macros, TEXT.RSC, wrapping), names, msgbox, title menu, chargen, HUD,
      items (equip.c), paperdoll, inventory screen and shop trades (inven.c); next spells,
      save/load, monsters/AI, combat, automap, talk, quests
- [ ] parity traces
