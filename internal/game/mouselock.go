package game

import "github.com/retro-ware/dagger-go/internal/xn"

// mouseLockUpdate asks the host to lock the pointer this frame when the game reads the
// mouse's motion rather than its cursor: mouse look (control mode 1 with the cursor off),
// the free look, and a weapon swing (the attack key held). Elsewhere the cursor follows the
// OS pointer. (port: the DOS driver gave motion and a cursor at once; a modern pointer is
// either locked or free.)
func mouseLockUpdate() {
	if GameMode != 0 || ScreenActive != 0 || GameStateFlags&0x24 != 0 {
		return
	}
	if Controls.MouseControlMode == 1 && ViewCursorActive == 0 || MiscStateFlags&0x40 != 0 ||
		KeyActionHeld(33) {
		xn.LockPointer()
	}
}

var (
	worldShown uint32 // xn.Shown after the last world frame's present
	uiWasOpen  bool   // a screen was over the world since the world last ran
	lookEarly  bool   // this frame's mouse turn came before its drawing (Enh.MouseLook)
)

// uiResume takes up the world after a screen over it closed, one run each frame (GameMode or
// ScreenActive set) or one with a loop of its own (seen by its presents): the keys still held
// from it (the Enter that closed a prompt) don't act again, the mouse's motion over it doesn't
// turn the view, and with Enh.MouseLook the mouse looks around again. (port: the game's own
// edges of the keys kept the Enter that ended the save's name for the world, which turned the
// cursor on.)
func uiResume() {
	if xn.Shown != worldShown {
		uiWasOpen = true
	}
	if GameMode != 0 || ScreenActive != 0 {
		uiWasOpen = true
		return
	}
	if !uiWasOpen {
		return
	}
	uiWasOpen = false
	for k := range KeyWasDown {
		KeyWasDown[k] = xn.KeyDown[k]
	}
	xn.MouseReadMotion()
	if Enh.MouseLook {
		ViewCursorActive = 0
	}
}
