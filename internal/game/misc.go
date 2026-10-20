package game

import (
	"strings"

	"github.com/memmaker/dagger-go/internal/xn"
)

// Small ports from several C files: the info popup, rain and snow, the guards, the
// pedestrians' sprites, trespassing and FLATS.CFG.

const (
	strFlatsCfg = 0x17531A
)

// InfoPopupUpdate draws the info popup (lines split by CR) and closes it on a click.
func InfoPopupUpdate() {
	if InfoPopupText == "" {
		return
	}
	lines := strings.Split(InfoPopupText, "\r")
	maxWidth := int32(0)
	for _, l := range lines {
		if w := int32(int16(FontTextWidth(l))); w > maxWidth {
			maxWidth = w
		}
	}
	n := int32(len(lines))
	maxWidth = (maxWidth + 10) / 2
	xn.DarkenRect(160-maxWidth, 100-n*5-5, maxWidth*2, n*10+10)
	x, y := 160-maxWidth+5, 100-n*5
	for _, l := range lines {
		TextDraw(l, x, y)
		y += 10
	}
	if ScreenActive == 1 && xn.MouseButtons&1 == 0 && xn.MouseButtonsPrev&1 != 0 {
		ScreenActive = InfoPopupSaved
		InfoPopupText = ""
	}
}

// WeatherDrawPrecipitation draws the rain or snow outdoors.
func WeatherDrawPrecipitation() {
	climate := ClimateCategory()
	if WorldRunning == 0 || PlayerEnvironment != 1 {
		return
	}
	weather := int32(uint8(ClimateWeathers[climate]))
	if o := Regions[CurrentRegion].PrecipitationOverride; o != 0 {
		weather = int32(o) - 1
	}
	draw := xn.SkyDrawSnow
	switch weather & 127 {
	case 5:
	case 4:
		draw = xn.SkyDrawRain
	default:
		return
	}
	saved := xn.ClipBottom
	if GameSettings.ViewFlags&1 != 0 {
		xn.ClipBottom = 199
	} else {
		xn.ClipBottom = int32(parseImg(HudBarImage).Y) - 2
	}
	draw()
	xn.ClipBottom = saved
}

// GuardsSummon calls the guards: now, from the pedestrians (guards and those looking away)
// or out of nowhere; otherwise when the crime was seen, soon.
func GuardsSummon(now int32) {
	if PlayerEnvironment == 3 || CreatureCount > 10 {
		return
	}
	if now != 0 {
		count := int32(0)
		for i := int32(0); i < PeopleCount; i++ {
			p := PeopleList[i]
			if p == nil {
				continue
			}
			if !IsGuardSprite(p) {
				angle := xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, p.X, p.Z)
				var dir int32
				if AiAngleDiff(int32(PlayerObject.Yaw), angle, &dir) < 600 {
					continue
				}
			}
			if uint8(Rand()&3) == 0 || IsGuardSprite(p) {
				count++
				GuardSpawn(p)
				PedestrianPlace(p)
			}
		}
		if count == 0 {
			count = RandRange(2, 5)
			for i := int32(0); i < count; i++ {
				GuardSpawn(nil)
			}
		}
		return
	}
	if PeopleWitnessFlags[0]&2 != 0 {
		for i := int32(0); i < PeopleCount; i++ {
			if p := PeopleList[i]; p != nil && IsGuardSprite(p) {
				GuardSpawn(p)
				PedestrianPlace(p)
			}
		}
		return
	}
	if PeopleWitnessFlags[0]&1 != 0 {
		GuardsTimer = RandRange(5, 10) * int32(xn.TimerFPS)
	}
}

// PedestrianPickSprite dresses a pedestrian for the climate (one in 32 is a beggar-like 399).
func PedestrianPickSprite(p *Record) {
	kind := ClimateCategory()
	flip := Rand() & 1
	if Rand()&31 == 0 {
		p.Image = 51072
		p.Flags &^= 0x4000
		return
	}
	k := uint32(U8At(addrClimatePeopleKind + uint32(kind)))
	p.Image = U16At(addrPedestrianSprites+(k*8+uint32(flip)*4+uint32(Rand()&3))*2) << 7
	if flip != 0 {
		p.Flags |= 0x4000
		if p.Person != nil {
			p.Person.Flags |= 16
		}
	}
}

// TownMapAreaClear says whether the town map is clear around a point (no buildings near).
func TownMapAreaClear(x, y int32) bool {
	x >>= 6
	y >>= 6
	stride := int32(CurrentLocation.Height) << 6
	y = stride - y - 1
	i := (int32(CurrentLocation.Width)<<6)*y + x
	px := func(j int32) byte {
		if j < 0 || j >= int32(len(TownMapPixels)) {
			return 0 // port: the original reads past the map's edges
		}
		return TownMapPixels[j]
	}
	blocked := byte(0)
	for _, d := range []int32{0, -stride, stride} {
		blocked |= px(i+d) | px(i+d-1) | px(i+d+1)
	}
	return blocked == 0
}

// FlatsCfgLoad reads FLATS.CFG: the people flats' names, sexes and portraits.
func FlatsCfgLoad() {
	raw := DiskReadFile(S(strFlatsCfg))
	c := &cfgCursor{Text: raw, Left: int32(len(raw))}
	FlatsCfg = FlatsCfg[:0]
	for {
		a := c.ReadNumber()
		if a == 100000 {
			return
		}
		b := c.ReadNumber()
		var f FlatCfg
		f.Image = uint16(a<<7 | b)
		copy(f.Name[:30], c.ReadLine())
		line := c.ReadLine() + "\x00\x00"
		i := 0
		if line[0] == '?' {
			f.Flags |= 2
			i = 1
		}
		if line[i] == '2' {
			f.Flags |= 1
		}
		f.Pad07 = uint8(c.ReadNumber())
		f.Pad08 = uint8(c.ReadNumber())
		f.Face = uint16(c.ReadNumber())
		FlatsCfg = append(FlatsCfg, f)
	}
}

// InteriorRestrictionAtPlayer is the restriction of the interior spot nearest the player:
// mode 2 its high byte.
func InteriorRestrictionAtPlayer(mode int32) int32 {
	o := PlayerObject.Parent
	for o.Type != 1 && o.Flags&1 == 0 {
		o = o.Parent
	}
	if o.Type != 43 {
		return 0
	}
	dx, dy, dz := PlayerObject.X-o.X, PlayerObject.Y-o.Y, PlayerObject.Z-o.Z
	shift := uint(0)
	if mode == 2 {
		shift = 8
	}
	o = o.Children
	for o.Type != 43 {
		o = o.Next
	}
	best, result := int32(100000), int32(0)
	for i, e := range o.Block.Section3 {
		if i >= int(o.Block.Section3Count) {
			break
		}
		d := xn.ApproxHypot(e.Y-dy, xn.ApproxDist2D(dx, dz, e.X, e.Z))
		if d < best {
			best = d
			result = int32(uint8(e.Data >> shift))
		}
	}
	return result
}

// BuildingAccessLevel is the access a rented building gives (nil: the player's building).
func BuildingAccessLevel(b *Building) int32 {
	if b == nil {
		b = ObjectBuilding(PlayerObject)
	}
	if b == nil {
		return 0
	}
	if b.Flags&1 != 0 && b.RentExpires > uint32(GameMinutes) {
		return int32(b.AccessLevel)
	}
	return 0
}

// UnequipObject takes the object out of the player's equipment.
func UnequipObject(o *Record) {
	for i := range PlayerCharacter.Equipped {
		if PlayerCharacter.Equipped[i] == o {
			PlayerCharacter.Equipped[i] = nil
		}
	}
}
