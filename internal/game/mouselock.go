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
