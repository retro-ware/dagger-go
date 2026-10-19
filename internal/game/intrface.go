package game

import "github.com/memmaker/dagger-go/internal/xn"

// intrface.c: the cursor, steering by the view's nine regions, the player's movement.

// MoveRequest is a move for the collision code to try (C's struct move_request).
type MoveRequest struct {
	X, Y, Z             int32
	AngleX, Yaw, AngleZ int32
	Probe               *xn.CollideProbe
	Flags               uint16
}

// PickResult is what the view has under a point (C's struct pick_result).
type PickResult struct {
	Flags           int32 // 1 hit, 2 a sprite, 4 a model, 8 a block model
	Object          *Record
	Plane           int32
	BlockModelIndex int16
	ModelID         int16
	Variant         int16
}

var (
	CursorRegionImages     []byte
	CursorArrowImage       []byte
	CursorSavedBackground  [100]byte
	CursorSavedX           int32
	CursorSavedY           int32
	ViewWidth, ViewHeight  int16
	SteerRegionWidth       int16
	SteerRegionHeight      int16
	SteerRowY1, SteerRowY2 int16
	SteerColX1, SteerColX2 int16
	SteerWeightUp          int16
	SteerWeightDown        int16
	SteerWeightLeft        int16
	SteerWeightRight       int16
	SteerKeyRegion         int16
	SteerStartTick         uint32
	SteerTicks             int32
	SteerTurnSpeedMax      int32
	TurnThisFrame          int32
	PlayerSpeed            int16
	PlayerBaseSpeed        int16
	PlayerMomentum         int32
	MoveAngleOffset        int32
	MoveDistanceTotal      int32
	MoveDxTotal            int32
	MoveDzTotal            int32
	MoveLastTick           uint32
	MoveLastMinute         uint32
	MoveRemainderX         int32
	MoveRemainderZ         int32
	MoveSettleFrames       int32
	VerticalVelocity       int32
	JumpVelocity           int32
	CeilingHeight          int32
	CollideHeight          int32
	PlayerClimbing         int16
	PlayerJumping          int16
	PlayerOnGround         int8
	InDungeonWater         int8
	CameraRollDisabled     int8
	CollideLanded          int8
	MouseTurnRate          uint8
	RidingSound            int32 = -1
	RidingSoundID          int32
	FloorObject            *Record
	MovingModel            *Record
	SpellReadyMissile      *Record
	CollideFlags           uint16 // 1 on a floor, 2 a wall, 4 climbing probe, 16 falling
	CollideWallCos         uint16
	CollideFloorHit        *xn.CollideHitRecord
	CollideSideHit         *xn.CollideHitRecord
	collideFloorHitCopy    xn.CollideHitRecord
	collideSideHitCopy     xn.CollideHitRecord
	PlayerMoveRequest      MoveRequest
	PlayerProbeCrouched    xn.CollideProbe
	PlayerProbeClimb       xn.CollideProbe

	// steerRegionHandlers are the nine view regions' steering (0xCAF00).
	steerRegionHandlers = [9]func(){
		SteerForwardLeft, SteerForward, SteerForwardRight, SteerTurnLeft, SteerCenter,
		SteerTurnRight, SteerSlideLeft, SteerBackward, SteerSlideRight,
	}
)

const textClimbingMode = 0x1845C8 // a pointer to "Climbing mode."

// IntrfaceInit loads the cursors.
func IntrfaceInit() {
	CursorRegionImages = DiskReadFile("pnter.cif")
	CursorArrowImage = DiskReadFile("arrow.raw")
	clear(ScratchBuffer[:256])
	for i := 0; i < 10; i++ {
		copy(ScratchBuffer[i<<4:i<<4+10], CursorArrowImage[i*10:])
	}
	xn.SetCursorImage(append([]byte(nil), ScratchBuffer[:256]...), 0, 0)
	IntrfaceSetRegions()
	ScreenActive = 0
}

// IntrfaceSetRegions splits the view into its 3 x 3 steering regions.
func IntrfaceSetRegions() {
	SteerRegionWidth = ViewWidth / 3
	SteerRegionHeight = ViewHeight / 3
	SteerRowY1 = ViewHeight/3 + int16(xn.ClipTop)
	SteerRowY2 = ViewHeight/3*2 + int16(xn.ClipTop)
	SteerColX1 = ViewWidth/3 + int16(xn.ClipLeft)
	SteerColX2 = ViewWidth/3*2 + int16(xn.ClipLeft)
}

// IntrfaceFree frees the cursors.
func IntrfaceFree() {
	CursorRegionImages = nil
	CursorArrowImage = nil
}

// CursorDrawArrow draws the arrow cursor, keeping what it covers.
func CursorDrawArrow() {
	xn.GetRect(int32(xn.MouseX), int32(xn.MouseY), 10, 10, CursorSavedBackground[:], 0)
	CursorSavedX, CursorSavedY = int32(xn.MouseX), int32(xn.MouseY)
	xn.DrawImageTransparent(int32(xn.MouseX), int32(xn.MouseY), 10, 10, CursorArrowImage)
	xn.MouseReadMotion()
	MouseMotionX, MouseMotionY = 0, 0
}

// CursorRestoreBackground puts back what the arrow covered.
func CursorRestoreBackground() {
	xn.DrawImageTransparent(CursorSavedX, CursorSavedY, 10, 10, CursorSavedBackground[:])
}

// PlayerAddPush pushes the player forward by distance.
func PlayerAddPush(distance int32) {
	GameStateFlags |= 16
	PlayerObject.Yaw = int16(uint16(PlayerObject.Yaw) &^ 0xF800)
	dx, dz := xn.YawOffsetXZ(int32(PlayerObject.Yaw), distance<<7)
	PlayerMomentum = distance
	MoveDistanceTotal += distance
	MoveDxTotal += dx
	MoveDzTotal += dz
}

// PlayerAddPushAngle pushes the player at an angle to their heading.
func PlayerAddPushAngle(distance, yawOffset int32) {
	saved := PlayerObject.Yaw
	PlayerObject.Yaw += int16(yawOffset)
	PlayerAddPush(distance)
	PlayerObject.Yaw = saved
}

func turnLeft() int32 {
	return -((FrameTicks * ((int32(SteerWeightLeft) << 6) * SteerTicks) / 1000) / 256)
}

func turnRight() int32 {
	return (FrameTicks * ((int32(SteerWeightRight) << 6) * SteerTicks) / 1000) / 256
}

func SteerForwardLeft() {
	xn.SnowTurnShift = 8
	TurnThisFrame = turnLeft()
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightUp) / 256)
}

func SteerForward() {
	TurnThisFrame = 0
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightUp) / 220)
}

func SteerForwardRight() {
	xn.SnowTurnShift = -8
	TurnThisFrame = turnRight()
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightUp) / 256)
}

func SteerTurnLeft() {
	xn.SnowTurnShift = 8
	TurnThisFrame = turnLeft()
	PlayerSpeed = 0
}

func SteerCenter() {
	PlayerSpeed = 0
	MoveAngleOffset = 0
	TurnThisFrame = 0
	if xn.MouseButtons&1 == 0 {
		return
	}
	ClickActivate(false)
}

func SteerTurnRight() {
	xn.SnowTurnShift = -8
	TurnThisFrame = ((int32(SteerWeightRight) << 6) * SteerTicks * FrameTicks / 1000) / 256
	PlayerSpeed = 0
}

func SteerSlideLeft() {
	xn.SnowTurnShift = 4
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightLeft) / 256)
	MoveAngleOffset = -512
}

func SteerBackward() {
	MoveAngleOffset = 1024
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightDown) / 256)
}

func SteerSlideRight() {
	xn.SnowTurnShift = -4
	MoveAngleOffset = 512
	PlayerSpeed = int16(int32(PlayerSpeed) * int32(SteerWeightRight) / 256)
}

// ClickActivate uses what is under the cursor (or the view's centre), else fires the
// readied spell.
func ClickActivate(atViewCentre bool) {
	var pick PickResult
	if WorldRunning == 0 {
		return
	}
	if !atViewCentre {
		if Controls.MouseControlMode == 1 && ViewCursorActive != 0 {
			EnginePickObject(int32(xn.MouseX), int32(xn.MouseY), &pick)
		} else {
			EnginePickObject(int32(xn.MouseX)+6, int32(xn.MouseY)+6, &pick)
		}
	} else {
		xn.MouseX, xn.MouseY = int16(xn.CamCentreX), int16(xn.CamCentreY)
		EnginePickObject(xn.CamCentreX, xn.CamCentreY, &pick)
	}
	if pick.Flags&1 != 0 {
		ClickWorldObject(&pick, pick.Object)
		return
	}
	if SpellReadyMissile == nil {
		return
	}
	CastFireMissile(SpellReadyMissile)
	SpellReadyMissile = nil
}

// IntrfaceKeyRegion is the steering region the movement keys ask for, -1 none.
func IntrfaceKeyRegion() int16 {
	SteerWeightUp, SteerWeightLeft, SteerWeightDown, SteerWeightRight = 255, 255, 255, 255
	switch {
	case KeyActionHeld(3):
		return 6
	case KeyActionHeld(2) && KeyActionHeld(10):
		return 6
	case KeyActionHeld(5):
		return 8
	case KeyActionHeld(4) && KeyActionHeld(10):
		return 8
	case KeyActionHeld(0) && KeyActionHeld(4):
		return 2
	case KeyActionHeld(0) && KeyActionHeld(2):
		return 0
	case KeyActionHeld(0):
		return 1
	case KeyActionHeld(1):
		return 7
	case KeyActionHeld(4) && !KeyActionHeld(10):
		return 5
	case KeyActionHeld(2) && !KeyActionHeld(10):
		return 3
	}
	return -1
}

// ObjectApplyGravity drops the object by its fall speed.
func ObjectApplyGravity(o *Record, c *Character) {
	if DungeonWaterLevel != 10000 && c == PlayerCharacter {
		if o.Y-50 > DungeonWaterLevel {
			if InDungeonWater == 0 {
				SoundPlay(86, o, 100)
			}
			InDungeonWater = 1
			VerticalVelocity = 0
			return
		}
	}
	InDungeonWater = 0
	if c.Conditions&0x8 != 0 {
		return
	}
	if c == PlayerCharacter && PlayerMotionFlags&0x20 != 0 {
		VerticalVelocity = 0
		return
	}
	if c.Conditions&0x4000 != 0 {
		VerticalVelocity = 15360
	}
	if o.Y-80 <= CeilingHeight && VerticalVelocity < 0 {
		VerticalVelocity = 0
		return
	}
	VerticalVelocity += FrameTicks * 100352 / 1000
	o.Y += VerticalVelocity * FrameTicks / 1000 / 256
}

// PlayerComputeJumpVelocity is the jump's impulse from strength, agility and jumping.
func PlayerComputeJumpVelocity() {
	JumpVelocity = (int32(PlayerCharacter.Attributes[6])+int32(PlayerCharacter.Attributes[0]))/2 + 50
	JumpVelocity += int32(PlayerCharacter.Skills[3].Value) * JumpVelocity / 100
	JumpVelocity = -JumpVelocity
	JumpVelocity <<= 8
}

// CameraRollTurn leans the camera into a turn.
func CameraRollTurn(target int32) {
	if CameraRollDisabled != 0 {
		return
	}
	c := CameraObject
	if target < 0 {
		c.AngleZ -= 8
		c.AngleZ = int16(uint16(c.AngleZ) &^ 0xF800)
		if int32(c.AngleZ) < target+2047 {
			c.AngleZ = int16(target + 2047)
		}
		return
	}
	c.AngleZ += 8
	if int32(c.AngleZ) <= target {
		return
	}
	c.AngleZ = int16(target)
}

// CameraRollRecover levels the camera.
func CameraRollRecover() {
	c := CameraObject
	if c.AngleZ == 0 {
		return
	}
	if c.AngleZ < 1024 {
		c.AngleZ -= 8
		if c.AngleZ < 0 {
			c.AngleZ = 0
		}
		return
	}
	c.AngleZ += 8
	if c.AngleZ <= 2047 {
		return
	}
	c.AngleZ = 0
}

// PlayerMotionClearBit6 clears motion flag 0x40.
func PlayerMotionClearBit6(int32) { PlayerMotionFlags &= 191 }

// VectorYaw is the vector's heading.
func VectorYaw(v *xn.Vec3) int32 {
	sine := (v.X << 8) / xn.Isqrt(v.X*v.X+v.Z*v.Z)
	var yaw int32
	if v.X > 0 {
		yaw = sine * 511 / 256
		if v.Z < 0 {
			yaw = 512 - yaw + 512
		}
	} else {
		yaw = sine*511/256 + 2047
		if v.Z < 0 {
			yaw = 1536 - (yaw - 1536)
		}
	}
	return yaw & 2047
}

// IntrfacePollControls reads the mouse and keys.
func IntrfacePollControls() {
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	SwiftPoll()
	SteerKeyRegion = IntrfaceKeyRegion()
	if SteerKeyRegion != -1 {
		r := SteerKeyRegion
		if r == 0 || r == 2 || r == 3 || r == 5 {
			return
		}
	}
	SteerStartTick = xn.BIOSTicks()
}

// IntrfaceRegionAt is the steering region at (x, y) and its top left.
func IntrfaceRegionAt(x, y int32) (region, regionX, regionY int32) {
	switch {
	case int32(SteerRowY1) > y:
		region, regionY = 0, xn.ClipTop
	case int32(SteerRowY2) > y:
		region, regionY = 3, int32(SteerRowY1)
	default:
		region, regionY = 6, int32(SteerRowY2)
	}
	switch {
	case int32(SteerColX1) > x:
		regionX = xn.ClipLeft
	case int32(SteerColX2) > x:
		region++
		regionX = int32(SteerColX1)
	default:
		region += 2
		regionX = int32(SteerColX2)
	}
	return
}

// IntrfaceSteer sets the speed and turn from the mouse's place in its region or the keys.
func IntrfaceSteer(region, regionX, regionY int32) {
	if xn.MouseButtons&1 != 0 || SteerKeyRegion != -1 {
		if SteerKeyRegion == -1 {
			SteerTicks = SteerTurnSpeedMax
			w, h := int32(SteerRegionWidth), int32(SteerRegionHeight)
			mx, my := int32(xn.MouseX)-regionX, int32(xn.MouseY)-regionY
			SteerWeightRight = int16((mx << 8) / w)
			SteerWeightDown = int16((my << 8) / h)
			SteerWeightLeft = int16(((w - mx) << 8) / w)
			SteerWeightUp = int16(((h - my) << 8) / h)
		} else {
			SteerTicks = int32(xn.BIOSTicks() - SteerStartTick)
			if SteerTicks > SteerTurnSpeedMax {
				SteerTicks = SteerTurnSpeedMax
			}
		}
	}
	MoveAngleOffset = 0
	PlayerSpeed = PlayerBaseSpeed
	if PlayerCharacter.Conditions&1 == 0 &&
		((Controls.MouseControlMode != 1 && xn.MouseButtons&1 != 0) || SteerKeyRegion != -1) {
		steerRegionHandlers[region]()
	} else {
		PlayerSpeed = 0
		TurnThisFrame = 0
		MoveAngleOffset = 0
	}
	PlayerMoving = b2u8(PlayerSpeed > 2)
}

// ClimbAngleOk is whether a wall that steep can be climbed.
func ClimbAngleOk(wallCos int32) bool { return wallCos == 0 || (wallCos > 250 && wallCos < 265) }

func playerAirborne() bool {
	return PlayerOnGround == 0 && InDungeonWater == 0 && PlayerMotionFlags&0x20 == 0
}

// PlayerMovementUpdate steers, moves, climbs, jumps and drops the player for a frame.
func PlayerMovementUpdate() {
	if MoveSettleFrames != 0 {
		MoveSettleFrames--
		return
	}
	WorldStateFlags &= 223
	SteerTurnSpeedMax = int32(MouseTurnRate&127)*2 + 4
	PlayerMouseLook()
	IntrfacePollControls()
	PlayerMoving = 0
	if HudUpdate() {
		return
	}
	region, regionX, dy := IntrfaceRegionAt(int32(xn.MouseX), int32(xn.MouseY))
	CursorDraw(int16(region))
	if ScreenActive != 0 {
		return
	}
	if KeyActionHeld(22) || (MiscStateFlags&0x40 != 0 && xn.MouseButtons&1 != 0) {
		return
	}
	CollideFloorHit, CollideSideHit = nil, nil
	startX, startZ := PlayerObject.X, PlayerObject.Z
	prevVelocity := VerticalVelocity
	if SteerKeyRegion != -1 {
		region = int32(SteerKeyRegion)
	}
	if PlayerCharacter.Conditions&0x8 != 0 {
		PlayerOnGround = 1
	}
	airborne := playerAirborne()
	var distance int32
	if airborne {
		TurnThisFrame = 0
		distance = PlayerMomentum
	} else {
		IntrfaceSteer(region, regionX, dy)
		distance = int32(PlayerSpeed)
		PlayerMomentum = distance
	}
	if UIModeFlags&2 != 0 {
		UIModeFlags &= 253
		return
	}
	if TurnThisFrame != 0 {
		CameraRollTurn(TurnThisFrame / 4)
		CameraObject.Yaw += int16(TurnThisFrame)
		PlayerObject.Yaw = CameraObject.Yaw
		TurnThisFrame = 0
	} else {
		CameraRollRecover()
	}
	stepSize := int32(10)
	if PlayerMotionFlags&4 != 0 {
		stepSize = 5
	}
	blocked := false
	if !airborne && Controls.MouseControlMode == 1 && KeyActionHeld(23) {
		PlayerMomentum >>= 1
		PlayerMomentum--
		distance >>= 1
	}
	for !blocked && distance > 0 {
		var step int32
		if distance < stepSize {
			step = distance
			distance = 0
		} else {
			distance -= stepSize
			step = stepSize
		}
		blocked = PlayerTryMove(step)
		if CollideFlags&16 == 0 && CollideFlags&1 != 0 {
			LinksTrigger(FloorObject, 1)
		}
		if CollideFlags&2 != 0 {
			LinksTrigger(MovingModel, 3)
		}
	}
	if distance == 0 && FloorObject != nil && LinksObjectMotion(FloorObject) {
		PlayerTryMove(0)
	}
	dy = 0
	if CollideFlags&2 != 0 && PlayerClimbing == 0 && ClimbAngleOk(int32(CollideWallCos)) &&
		approxDist2D(PlayerObject.X, PlayerObject.Z, startX, startZ) < 5 &&
		PlayerCharacter.Conditions&0x8 == 0 && PlayerCharacter.Flags&1536 == 0 {
		if xn.BIOSTicks()-MoveLastTick > 14 {
			if PlayerMotionFlags&0x20 == 0 {
				HudMessageAdd(SP(textClimbingMode))
				SkillAddUses(18, 1)
			}
			PlayerMotionFlags |= 32
			dy = -int32(PlayerSpeed) / 3
			if uint32(GameMinutes-MoveLastMinute) > 1 {
				SkillAddUses(18, 1)
				MoveLastMinute = GameMinutes
				climbChance := int32(PlayerCharacter.Skills[18].Value)
				if PlayerCharacter.Race == 6 {
					climbChance += 30
				}
				if PlayerCharacter.Conditions&0x20000 == 0 && RandRange(1, 100) > 95 && RandRange(1, 100) > climbChance {
					PlayerMotionFlags &= 223
					PlayerClimbing = 1
				}
			}
		}
	} else {
		if PlayerMotionFlags&0x20 != 0 && PlayerClimbing == 0 && CollideFlags&2 == 0 {
			if !PlayerClimbProbe() {
				PlayerMotionFlags &= 223
			}
		}
		MoveLastTick = xn.BIOSTicks()
		MoveLastMinute = GameMinutes
	}
	if PlayerMotionFlags&0x20 != 0 || PlayerCharacter.Conditions&0x8 != 0 {
		VerticalVelocity = 0
	}
	if PlayerObject.Y-50 <= DungeonWaterLevel {
		InDungeonWater = 0
	}
	PlayerJumping = 0
	if ((xn.MouseButtons&3 == 3 && xn.MouseButtonsPrev&3 == 1) || KeyActionHeld(8)) && PlayerOnGround != 0 &&
		PlayerCharacter.Conditions&0x1 == 0 && PlayerCharacter.Flags&1536 == 0 {
		SkillAddUses(3, 1)
		impulse := JumpVelocity
		if PlayerCharacter.Conditions&0x10000 != 0 {
			impulse = JumpVelocity * 2
		}
		if PlayerClass.Flags&2 != 0 {
			impulse = JumpVelocity + JumpVelocity>>1
		}
		dy = -1
		VerticalVelocity += impulse
		PlayerJumping = 1
		FatigueAdd(-11)
		PlayerOnGround = 0
		CollideLanded = 1
	}
	if KeyActionHeld(6) && (PlayerCharacter.Conditions&0x8 != 0 || InDungeonWater != 0) && PlayerObject.Y-100 > CeilingHeight {
		if PlayerEnvironment != 1 || CollideHeight-1024 <= PlayerObject.Y {
			dy += FrameTicks * -80 / 1000
		}
	} else if KeyActionHeld(7) && (PlayerCharacter.Conditions&0x8 != 0 || InDungeonWater != 0) {
		dy += FrameTicks * 80 / 1000
	}
	if InDungeonWater != 0 && ObjectWeight(PlayerEntity) > 250 && !KeyActionHeld(7) && PlayerCharacter.Conditions&1048584 == 0 {
		dy += FrameTicks * 80 / 1000
	}
	savedY := PlayerObject.Y
	airborne = playerAirborne()
	if CollideFlags&16 != 0 || airborne || (DungeonWaterLevel != 10000 && PlayerObject.Y-50 > DungeonWaterLevel) {
		savedY = PlayerObject.Y
		ObjectApplyGravity(PlayerObject, PlayerCharacter)
		dy += PlayerObject.Y - savedY
		PlayerObject.Y = savedY
	} else {
		PlayerClimbing = 0
		VerticalVelocity = 0
	}
	PlayerTryMoveVertical(dy)
	if PlayerOnGround != 0 {
		VerticalVelocity = 0
	}
	if VerticalVelocity < 0 && PlayerJumping != 0 && PlayerObject.Y-99 < CeilingHeight {
		PlayerObject.Y = savedY
	}
	if CollideFloorHit != nil {
		PlayerClimbing = 0
		if PlayerOnGround != 0 {
			VerticalVelocity = 0
		}
		collideFloorHitCopy = *CollideFloorHit
		CollideFloorHit = &collideFloorHitCopy
	}
	if CollideSideHit != nil {
		collideSideHitCopy = *CollideSideHit
		CollideSideHit = &collideSideHitCopy
	}
	if VerticalVelocity == 0 && prevVelocity != 0 {
		fall := prevVelocity/256/40 - 7
		if fall > 0 {
			fall = fall * fall / 2
			DamageApply(PlayerEntity, fall, nil)
			snd := int32(359)
			if PlayerEnvironment != 3 {
				snd = 360
			}
			SoundPlay(snd, PlayerEntity, 100)
		}
	}
	if Controls.MouseControlMode == 1 && ScreenActive == 0 {
		if ViewCursorActive != 0 && xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev&1 == 0 {
			ClickActivate(false)
		}
		if KeyActionPressed(18) {
			ClickActivate(true)
		}
	} else if ScreenActive == 0 {
		if xn.MouseButtons&2 != 0 && DisplayFlags&0x40 == 0 {
			ClickActivate(false)
		}
	}
	PlayerHorseSounds(PlayerMomentum)
	if DungeonWaterLevel == 10000 || PlayerObject.Y-76 <= DungeonWaterLevel || Rand() >= 100 {
		return
	}
	SoundPlay(384, PlayerObject, 100)
}

// PlayerHorseSounds plays the horse's or cart's hooves at the speed.
func PlayerHorseSounds(speed int32) {
	if PlayerCharacter.Flags&1536 == 0 {
		return
	}
	sound := int32(365)
	if int32(PlayerBaseSpeed)-int32(PlayerBaseSpeed)>>2 < speed {
		sound = 366
	}
	if PlayerCharacter.Flags&1024 != 0 {
		sound = 372
	}
	if speed == 0 {
		if RidingSound == -1 {
			return
		}
		if SoundChannelDone(RidingSound) {
			return
		}
		SoundStopChannel(RidingSound)
		RidingSound = -1
		return
	}
	if RidingSound == -1 {
		RidingSound = SoundPlayLoop(sound, PlayerObject, 100)
	} else if sound != RidingSoundID {
		SoundStopChannel(RidingSound)
		RidingSound = SoundPlayLoop(sound, PlayerObject, 100)
	}
	RidingSoundID = sound
}

// PlayerHorseSoundsStop stops the hooves (sic: it forgets the channel before stopping it).
func PlayerHorseSoundsStop() {
	RidingSound = -1
	if SoundChannelDone(RidingSound) {
		return
	}
	SoundStopChannel(RidingSound)
}

// PlayerClimbProbe is whether there is still wall ahead to climb.
func PlayerClimbProbe() bool {
	x, z := xn.YawOffsetXZ((int32(PlayerObject.Yaw)+MoveAngleOffset)&2047, 1024)
	x += PlayerObject.X << 5
	z += PlayerObject.Z << 5
	x += MoveRemainderX
	z += MoveRemainderZ
	MoveRemainderX = x & 31
	MoveRemainderZ = z & 31
	r := &PlayerMoveRequest
	r.X, r.Y, r.Z = x/32, PlayerObject.Y-32, z/32
	r.AngleX, r.Yaw, r.AngleZ = int32(PlayerObject.AngleX), int32(PlayerObject.Yaw), int32(PlayerObject.AngleZ)
	if PlayerMotionFlags&4 != 0 {
		r.Probe = &PlayerProbeCrouched
	} else {
		r.Probe = &PlayerCollideProbe
	}
	if PlayerMotionFlags&0x20 != 0 {
		r.Probe = &PlayerProbeClimb
	}
	CollideFlags |= 4
	PlayerMotionFlags &= 223
	return CollideMovePlayer(PlayerObject, 0, r, 1) != 0
}

// ParseHexString reads a hex number (-1 for an empty string; only a lowercase 'a').
func ParseHexString(text string) int32 {
	if len(text) == 0 {
		return -1
	}
	place := int32(1)
	p := len(text) - 1
	value := hexDigitValue(text[p])
	for p != 0 {
		p--
		place <<= 4
		value += hexDigitValue(text[p]) * place
	}
	return value
}

func hexDigitValue(c byte) int32 {
	switch {
	case c >= '0' && c <= '9':
		return int32(c) - 48
	case c >= 'A' && c <= 'Z':
		return int32(c) - 55
	case c == 'a':
		return int32(c) - 87
	}
	return 0
}
