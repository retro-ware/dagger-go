package game

import "github.com/memmaker/dagger-go/internal/xn"

// intro.c: the logo and intro movies.

func clearScreens() {
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
}

// IntroPlayLogo plays the Bethesda logo.
func IntroPlayLogo() {
	waitMouseUp()
	xn.VidPlay(DiskResolvePath("anim0001.vid"), 0, 0, true)
	clearScreens()
	waitMouseUp()
	clear(xn.VGA[:64000])
	PaletteRestore()
}

// IntroPlayMovie plays the new game's intro.
func IntroPlayMovie() {
	clear(xn.VGA[:64000])
	waitMouseUp()
	xn.VidPlay(DiskResolvePath("anim0000.vid"), 0, 0, true)
	clearScreens()
	waitMouseUp()
	xn.VidPlay(DiskResolvePath("anim0011.vid"), 0, 0, true)
	clearScreens()
	waitMouseUp()
	xn.VidPlay(DiskResolvePath("dag2.vid"), 32, 0, true)
	clearScreens()
	PaletteRestore()
}

// ChargenGiveStartingEquipment gives a new character their equipment.
func ChargenGiveStartingEquipment() { StartingEquipmentGive() }
