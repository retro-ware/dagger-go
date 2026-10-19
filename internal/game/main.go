package game

import (
	"fmt"
	"os"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// Main-loop state (main.c).
var (
	FrameCheckpoint  int32
	StartTick        int32
	StartTickPlus18  int32
	EngineRunning    int32
	FrameCounter     int32
	PlayerDeathTimer int32
	MouseMotionX     int16
	MouseMotionY     int16
	TextShadowColour uint8
	FogColour        uint8
	PoliticsNewYear  uint8
)

const strMustRunFromHardDrive = 0x170049

// Main is the game's main (FALL.EXE 0x10010). cfgPath is Z.CFG.
func Main(cfgPath string) {
	Srand(xn.BIOSTicks())
	StartTick = int32(xn.BIOSTicks())
	ConfigRead(cfgPath)
	if !dosfs.Exists(`ARENA2\COPYFILE.DAT`) {
		fmt.Print(S(strMustRunFromHardDrive))
		os.Exit(-1)
	}
	MouseSetBounds(0, 0, 319, 199)
	InitVideo()
	KludgePrintBuild(0)
	InitGameData()
	InitPalette()
	QuestInitRecordSizes()
	IntroPlayLogo()
	for n := 0; n < 12; n++ {
		PoliticsNewYear = b2u8(n == 11)
		FactionPoliticsUpdate(1)
		FactionPoliticsUpdate(2)
	}
	for {
		TitleMenu()
		StartTickPlus18 = int32(xn.BIOSTicks()) + 18
		TimePass(1)
		xn.TimerFPS = 23
		xn.RenderSetMode(8)
		EngineRunning = 1
		for {
			FrameCheckpoint = 0
			GameFrame()
			FrameCheckpoint = 1
			if PlayerDeathTimer < 0 {
				GameReset()
				PlayerDeathTimer = 0
				break
			}
			HudDrawHeadingStrip(0)
			xn.TimerFPSUpdate()
			FrameCheckpoint = 2
			MouseMotionX, MouseMotionY = xn.MouseReadMotion()
			PlayerMovementUpdate()
			KeysWorldActions()
			QuestDebugOverlay()
			FrameCheckpoint = 3
			TextShadowColour = FogColour
			xn.PresentInclusive(true)
			ScreenshotPoll()
			FrameCounter++
		}
	}
}

func b2u8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
