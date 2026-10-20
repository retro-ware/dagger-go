package game

import (
	"bytes"
	"strings"
)

// Music (run_00077B9E.c, sound.c): which song fits the place, the time and the weather.

const (
	addrMusicDungeonGM         = 0x1876AC // char *[15]
	addrMusicGeneralGM         = 0x1876E8
	addrMusicSneakingGM        = 0x187734
	addrMusicGuildGM           = 0x187750
	addrMusicBuildingsGM       = 0x187770
	addrMusicCourtGM           = 0x187784
	addrMusicDungeonFM         = 0x1877A0
	addrMusicGeneralFM         = 0x1877DC
	addrMusicSneakingFM        = 0x187828
	addrMusicGuildFM           = 0x187844
	addrMusicBuildingsFM       = 0x187864
	addrMusicCourtFM           = 0x187878
	addrMusicGuildFactionsGood = 0x1789E8 // [8]
	addrMusicGuildFactionsBad  = 0x1789F0 // [8]
	addrMusicSpecialDungeonIDs = 0x187894 // int16[10]
	addrDungeonSongs           = 0x1878AC // char *[10]
	addrDungeonSongsFM         = 0x1878D4
	strGshopHmi                = 0x1767E4
	strFmSqr2Hmi               = 0x1766F9
	strGmage3Hmi               = 0x1767EE
	strFmNite3Hmi              = 0x1767FA
	strMagic2Hmi               = 0x176807
	str23Hmi                   = 0x176813
	str23fmHmi                 = 0x17681A
	str17Hmi                   = 0x176823
	str17fmHmi                 = 0x17682A
	str06Hmi                   = 0x176833
	str06fmHmi                 = 0x17683A
)

var (
	// MusicUsesFM picks the FM songs (an FM synthesizer) over the General MIDI ones.
	MusicUsesFM uint8
	// MusicCurrent is the playing song's file name.
	MusicCurrent string
	// MusicPlayer plays an HMI song (nil: stops it), MusicVolumeSet sets its volume (0..128);
	// nil: no music device. The host sets them.
	MusicPlayer    func(hmi []byte)
	MusicVolumeSet func(volume int32)
	MidiBSA        int32

	musicDungeon, musicGeneral, musicSneaking uint32
	musicGuild, musicBuildings, musicCourt    uint32
)

func musicSong(table uint32, i int32) string { return SP(table + uint32(i)*4) }

// MusicSelectTables picks the FM or the General MIDI song tables.
func MusicSelectTables() {
	if MusicUsesFM != 0 {
		musicDungeon, musicGeneral, musicSneaking = addrMusicDungeonFM, addrMusicGeneralFM, addrMusicSneakingFM
		musicGuild, musicBuildings, musicCourt = addrMusicGuildFM, addrMusicBuildingsFM, addrMusicCourtFM
		return
	}
	musicDungeon, musicGeneral, musicSneaking = addrMusicDungeonGM, addrMusicGeneralGM, addrMusicSneakingGM
	musicGuild, musicBuildings, musicCourt = addrMusicGuildGM, addrMusicBuildingsGM, addrMusicCourtGM
}

func musicFM(gm, fm uint32) string {
	if MusicUsesFM == 0 {
		return S(gm)
	}
	return S(fm)
}

// MusicPlay starts a song unless it is already playing.
func MusicPlay(name string) {
	if !SoundEnabled || strings.EqualFold(MusicCurrent, name) {
		return
	}
	MusicStop()
	MusicCurrent = name
	if MusicPlayer != nil {
		MusicPlayer(ArchiveReadRecord(MidiBSA, ArchiveFindRecord(MidiBSA, name, 13), nil))
	}
}

// MusicStop stops the song.
func MusicStop() {
	if !SoundEnabled || MusicCurrent == "" {
		return
	}
	MusicCurrent = ""
	if MusicPlayer != nil {
		MusicPlayer(nil)
	}
}

// MusicUpdate restarted a finished song; the host's player starts it again itself.
func MusicUpdate() {}

// MusicPlayCourt plays the court's song.
func MusicPlayCourt() { MusicPlay(musicSong(musicCourt, 1)) }

// musicDungeonSong is a special dungeon's own song, or "". As in the original, the FM setting
// picks the General MIDI song and the other way round.
func musicDungeonSong() string {
	id := int16(LocationObject.ID >> 16)
	for i := uint32(0); i < 10; i++ {
		if id == int16(U16At(addrMusicSpecialDungeonIDs+i*2)) {
			if MusicUsesFM != 0 {
				return SP(addrDungeonSongs + i*4)
			}
			return SP(addrDungeonSongsFM + i*4)
		}
	}
	return ""
}

func musicPlayWeather(climate int32) {
	switch ClimateWeathers[climate] {
	case 0:
		MusicPlay(musicSong(musicGeneral, Rand()%7))
	case 1:
		MusicPlay(musicSong(musicGeneral, Rand()%9))
	case 2, 3, 6:
		MusicPlay(musicSong(musicGeneral, Rand()%5+7))
	case 4:
		MusicPlay(musicSong(musicGeneral, Rand()%3+12))
	case 5:
		MusicPlay(musicSong(musicGeneral, Rand()%3+15))
	}
}

// MusicChooseSong plays the song for where the player is.
func MusicChooseSong() {
	if WorldRunning == 0 || musicGeneral == 0 {
		return
	}
	seed := Rand()
	switch PlayerEnvironment {
	case 3:
		if DungeonBlockSpecial != 0 {
			MusicPlay(musicSong(musicGeneral, 10))
		} else if p := musicDungeonSong(); p != "" {
			MusicPlay(p)
		} else {
			Srand(uint32(int32(CurrentRegion)<<8 ^ int32(LocationObject.Image)))
			MusicPlay(musicSong(musicDungeon, Rand()%15))
		}
	case 1:
		climate := ClimateCategory()
		Srand(uint32(GameMinutes / 1440))
		inside := LocationContains(PlayerObject.X, PlayerObject.Z)
		kind := CurrentLocation.Kind
		switch {
		case IsDaytime == 0:
			MusicPlay(musicSong(musicCourt, Rand()%7))
		case !inside || kind != 4 && kind <= 9:
			musicPlayWeather(climate)
		case inside && (kind == 4 || kind >= 9):
			MusicPlay(musicSong(musicCourt, Rand()%7))
		default:
			musicPlayWeather(climate)
		}
	default:
		Srand(uint32(uint16(BuildingMusicSeed)))
		if Trespassing != 0 {
			MusicPlay(musicSong(musicSneaking, Rand()%7))
			Srand(uint32(seed))
			return
		}
		b := CurrentBuilding
		switch b.Type {
		case 0, 2, 3, 5, 6, 7, 8, 9, 10, 12, 13:
			MusicPlay(musicFM(strGshopHmi, strFmSqr2Hmi))
		case 11:
			if b.FactionID == 40 {
				if Rand()&1 != 0 {
					MusicPlay(musicFM(strGmage3Hmi, strFmNite3Hmi))
				} else {
					MusicPlay(musicFM(strMagic2Hmi, strFmNite3Hmi))
				}
			} else {
				MusicPlay(musicFM(str23Hmi, str23fmHmi))
			}
		case 14:
			if InKnightlyOrderHall != 0 {
				MusicPlay(musicFM(str17Hmi, str17fmHmi))
				break
			}
			idx := bytes.IndexByte(Bytes(addrMusicGuildFactionsGood, 8), byte(b.FactionID))
			if idx < 0 {
				idx = bytes.IndexByte(Bytes(addrMusicGuildFactionsBad, 8), byte(b.FactionID))
				if idx < 0 {
					MusicPlay(musicFM(str17Hmi, str17fmHmi))
					break
				}
			}
			MusicPlay(musicSong(musicGuild, int32(idx)))
		case 15:
			MusicPlay(musicSong(musicBuildings, int32(GameMinutes/1440%5)))
		case 16:
			Rand() // rand() % 1: one choice
			MusicPlay(musicFM(str06Hmi, str06fmHmi))
		default:
			MusicPlay(musicFM(str23Hmi, str23fmHmi))
		}
	}
	Srand(uint32(seed))
}
