package game

import "github.com/memmaker/dagger-go/internal/xn"

// The per-frame update and the world render (main.c).

var (
	GameFrameLastTick     int32
	GameModeUnderPopup    uint8
	PlayerMount           *Record
	InvRightContainer     *Record
	InvRightContainerBase *Record
	TradeContainer        Record
	GroundTextureArchive  int16
	NatureTextureArchive  int16
	SwayPhase             int32
	HeadBobOffset         int32
	ModelQueueCountA      int32
	ModelQueueCountB      int32
	ScreenShake           int32
	ScreenShakeTimer      int32
	ScreenShakeAngle      int32
	ScreenShakeSigns      int32
	ScreenShakePitch      int32
	ScreenShakeYaw        int32
)

const (
	swayYOffsets             = 0x1788E8 // int32[32]
	swayRollOffsets          = 0x178968 // int32[32]
	strInsufficientTexMemory = 0x1700A0
)

// GameFrame runs one frame of the world (game_frame).
func GameFrame() {
	MusicChooseSong()
	if FrameCounter&15 == 0 {
		MusicUpdate()
	}
	FrameCheckpoint = 99
	FrameTicksUpdate()
	xn.SkySnowUpdateSpeeds()
	FrameCheckpoint = 100
	UpdateFog()
	HeadBobUpdate()
	PlayerEntity.X = PlayerObject.X
	PlayerEntity.Y = PlayerObject.Y
	PlayerEntity.Z = PlayerObject.Z
	if PlayerMount != nil && PlayerOnGround != 0 && PlayerObject.Parent != PlayerMount {
		ObjectReparent(PlayerMount, PlayerObject)
	}
	FrameCheckpoint = 101
	LinksUpdate()
	FrameCheckpoint = 102
	SkyUpdate()
	FrameCheckpoint = 103
	WorldRender()
	FrameCheckpoint = 104
	if WorldRunning != 0 {
		WorldCollectObjects()
		PeopleSpawnTick()
	}
	FrameCheckpoint = 107
	xn.TextColour = 146
	xn.FontSelect(4)
	reset := GameModeUnderPopup != 4 || uint8(GameMode) != 8
	if reset && uint8(GameMode) != 4 {
		InvRightContainer = &TradeContainer
		InvRightContainerBase = &TradeContainer
	}
	LoanCollectorsUpdate()
	WeatherDrawPrecipitation()
	CastAnimUpdate()
	AmbientOutdoorSounds()
	GuardsTimerTick()
	FootstepSounds()
	TimeUpdateRealtime()
	FrameCheckpoint = 109
	SpellHudDrawIcons()
	HudDraw()
	FrameCheckpoint = 110
	if WorldRunning != 0 {
		AIUpdateCreatures()
		FrameCheckpoint = 111
		PeopleUpdate()
		FrameCheckpoint = 112
	}
	QuestsRunAll()
	FrameCheckpoint = 113
	KludgeMenuUpdate()
	PotionmakerUpdate()
	SpellmakerUpdate()
	SheetUpdate()
	InventoryFrame()
	StatusShow(0)
	SpellbookFrame()
	ItemmakerUpdate()
	BookUpdate()
	FrameCheckpoint = 114
	LogbookUpdate()
	BankFrame()
	TradeHaggleFrame()
	CourtFrame()
	LevelupCheck()
	TravelMapUpdate()
	SpfxPopupUpdate()
	TavernFrame()
	RepairMenuFrame()
	CovenMenuFrame()
	ServiceMenuFrame()
	ShelfBookListUpdate()
	MagicItemsFrame()
	OptionsFrame()
	FrameCheckpoint = 115
	DamageExpireDrainBonuses()
	TalkUpdate()
	TrainingUpdate()
	InfoPopupUpdate()
	MarqueeUpdate()
	FrameCheckpoint = 116
	AmbientDungeonSounds()
	QuestFacesDraw()
	QuestStartPending()
	QuestPromptAnswer()
	SoundUpdateChannels()
	GuildCheckInvitations()
	FrameCheckpoint = 117
	if PlayerMotionFlags&16 != 0 {
		HudMessages[0] = SP(textClimbingMode)
		HudMessageExpiry[0] = 2
	}
	CreaturesFindNearest()
	EffectsTick()
	PlayerFrameUpdate()
	RestUpdate()
	WeaponPlayerUpdate()
	BreathUpdate()
	HudMessagesDraw()
	ArrivalRoomMessages()
	FrameCheckpoint = 118
}

// WorldRender sets the camera from the player and renders the world, retrying once with a
// flushed texture cache.
func WorldRender() {
	if WorldRunning == 0 {
		return
	}
	retries := 0
	ModelCacheFlushCount = 0
	DoorsUpdate()
	for {
		xn.CamX = PlayerObject.X
		xn.CamZ = PlayerObject.Z
		eye := int32(72)
		if PlayerMotionFlags&4 != 0 {
			eye = 35
		}
		xn.CamY = PlayerObject.Y - eye - HeadBobOffset
		if PlayerCharacter.Flags&1536 != 0 {
			xn.CamY -= 50
		}
		if PlayerDeathTimer > 0 {
			xn.CamY = (xn.CamY-(PlayerObject.Y-20))*PlayerDeathTimer/1000 + PlayerObject.Y - 20
		}
		if PlayerSwimming != 0 && PlayerDeathTimer <= 0 {
			xn.CamY += 50
		}
		xn.CamPitch = (int32(CameraObject.AngleX) + ViewLookPitch) & 2047
		xn.CamYaw = (int32(CameraObject.Yaw) + ViewLookYaw) & 2047
		xn.CamRoll = int32(CameraObject.AngleZ)
		if InDungeonWater != 0 || PlayerSwimming != 0 {
			SwayPhase += int32(FrameTicks) << 11 / 1000
			SwayPhase &= 8191
			k := uint32(SwayPhase>>8) * 4
			if InDungeonWater != 0 && CeilingHeight+5 < xn.CamY {
				xn.CamY += int32(U32At(swayYOffsets+k))*2 - 3
			}
			xn.CamRoll += int32(U32At(swayRollOffsets + k))
		}
		if xn.DungeonWaterLevel != 10000 && abs32(xn.CamY-xn.DungeonWaterLevel) < 5 {
			xn.CamY -= 10
		}
		pitch, yaw := ScreenShakeOffset()
		xn.CamPitch = (xn.CamPitch + pitch) & 2047
		xn.CamYaw = (xn.CamYaw + yaw) & 2047
		FrameCheckpoint = 200
		xn.TexCacheBeginFrame()
		xn.RenderBeginFrame()
		xn.LightReset()
		xn.CamRotation = xn.MatFromAngles(xn.CamPitch, xn.CamYaw, xn.CamRoll)
		xn.CamViewMatrix = xn.CamScaleMatrix(&xn.CamRotation)
		FrameCheckpoint = 201
		WorldDrawObjects()
		FrameCheckpoint = 202
		if ModelCacheFlushCount != 0 {
			continue
		}
		if PlayerEnvironment == 1 {
			xn.WorldUpdate()
			WorldUpdateLocation()
			if ModelCacheFlushCount != 0 {
				continue
			}
			xn.WorldCellHeader.GroundArchive = uint16(GroundTextureArchive)
			xn.WorldCellHeader.NatureArchive = uint16(NatureTextureArchive)
			FrameCheckpoint = 203
			xn.TerrainDraw()
			FrameCheckpoint = 204
		}
		ModelQueueCountB = xn.ModelQueueCount
		FrameCheckpoint = 205
		var incomplete int32
		if PlayerEnvironment == 1 {
			incomplete = xn.RenderFrame(2)
		} else {
			incomplete = xn.RenderFrame(0)
		}
		FrameCheckpoint = 206
		ModelQueueCountA = xn.ModelQueueCount
		if incomplete == 0 && !xn.TexCacheFull {
			break
		}
		retries++
		if retries != 1 {
			FatalError(S(strInsufficientTexMemory))
		}
		xn.TexCacheFull = false
		xn.TexCacheFlush()
	}
	if xn.DungeonWaterLevel == 10000 || xn.CamY == xn.DungeonWaterLevel {
		return
	}
	xn.WaterDraw()
}

// ScreenShakeOffset steps the shake: the pitch and yaw to add this frame.
func ScreenShakeOffset() (pitch, yaw int32) {
	if ScreenShake == 0 {
		return 0, 0
	}
	ScreenShakeTimer -= int32(FrameTicks)
	if ScreenShakeTimer > 0 {
		return ScreenShakePitch, ScreenShakeYaw
	}
	ScreenShakeTimer = 60
	ScreenShake -= 2
	ScreenShakeAngle = (ScreenShakeAngle + 128) & 2047
	pitch = xn.MulSin(ScreenShake, ScreenShakeAngle)
	yaw = pitch
	if ScreenShakeSigns&1 != 0 {
		pitch = -pitch
	}
	if ScreenShakeSigns&2 != 0 {
		yaw = -yaw
	}
	ScreenShakePitch, ScreenShakeYaw = pitch, yaw
	return pitch, yaw
}

// ScreenShakeStart shakes the view with the given strength.
func ScreenShakeStart(strength int32) {
	ScreenShake = strength / 2
	if ScreenShake&1 != 0 {
		ScreenShake++
	}
	ScreenShakeSigns = Rand()
	ScreenShakeAngle = 0
	ScreenShakeTimer = 60
}

// ScreenShakeStop ends the shake.
func ScreenShakeStop() { ScreenShake = 0 }
