package game

import "github.com/memmaker/dagger-go/internal/xn"

// intro.c: the logo and intro movies.

func clearScreens() {
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
}

// SkipIntro (the -skipintro flag, a testing aid) skips the logo and the new game's movies.
var SkipIntro bool

// introVid plays a movie unless SkipIntro is set.
func introVid(name string, flags int32) {
	if !SkipIntro {
		xn.VidPlay(DiskResolvePath(name), flags, 0, true)
	}
}

// IntroPlayLogo plays the Bethesda logo.
func IntroPlayLogo() {
	waitMouseUp()
	introVid("anim0001.vid", 0)
	clearScreens()
	waitMouseUp()
	clear(xn.VGA[:64000])
	PaletteRestore()
}

// IntroPlayMovie plays the new game's intro.
func IntroPlayMovie() {
	clear(xn.VGA[:64000])
	waitMouseUp()
	introVid("anim0000.vid", 0)
	clearScreens()
	waitMouseUp()
	introVid("anim0011.vid", 0)
	clearScreens()
	waitMouseUp()
	introVid("dag2.vid", 32)
	clearScreens()
	PaletteRestore()
}

// ChargenGiveStartingEquipment gives a new character their equipment.
func ChargenGiveStartingEquipment() { StartingEquipmentGive() }
