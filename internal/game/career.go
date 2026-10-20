package game

import (
	"bytes"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// career.c: the background questions after the class (BIOGnnT0.TXT), their effects on the new
// character, and the biography (BIO.DAT) the character sheet shows.

const (
	addrCareerAnswerBoxes     = 0x179F74 // rects, 12 bytes each: [0] the question, then the answers
	addrCareerBackgroundIDs   = 0x179FF8 // [10]: the regions a background can be from
	addrCareerBioButtons      = 0x17A002 // [3] rects: page up, page down, done
	addrStrBiographyFile      = 0x170741 // "biog%02dt0.txt"
	addrStrBoyName            = 0x170773 // "%bn"
	addrStrGirlName           = 0x170777 // "%gn"
	careerBioPageLines        = 21
	careerBioImage            = "lgbk00i0.img"
	careerBioQuestionsImage   = "biog00i0.img"
	careerBioFile             = "bio.dat"
	careerQuestionCount       = 12
	careerPersonSlotsPerKind  = 8
	careerPersonCharacterSize = characterSize
)

var (
	CareerBioAsk        bool
	CareerRegion        int16 // scratch_190d68: the region the background is from
	CareerBioFlag       int16 // scratch_190d6a: cleared by an answer's '&'
	CareerAnswerCount   int16 // scratch_190d64
	CareerAnswer        int16 // scratch_190d66
	CareerQuestion      int32 // scratch_190cac: the question being answered
	careerAnswers       []byte
	careerText          []byte
	BioModifiers        int32
	CareerResistBonus   int32
	CareerToHitPenalty  int32
	CareerPoisonResist  int32
	CareerFatigueFlag   int32
	BioPersonCounts     [2]uint8
	CareerBioText       []byte
	CareerBioPage       int
	CareerBioLine       int32
	CareerBioLines      int32
	careerBioWindowShot []byte
)

func careerRect(base uint32, i int) Rect {
	a := base + uint32(i)*12
	return Rect{int16(U16At(a)), int16(U16At(a + 2)), int16(U16At(a + 4)), int16(U16At(a + 6))}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// atoiAt is atoi of the text at b.
func atoiAt(b []byte) int32 {
	n := 0
	for n < len(b) && n < 24 && b[n] != 0 && b[n] != '\n' {
		n++
	}
	return Atoi(string(b[:n]))
}

// CareerBackgroundSummary asks (or, not ask, answers at random) the class's background
// questions and writes the biography.
func CareerBackgroundSummary(classID int32, ask bool) {
	CareerBioAsk = ask
	ReputationBaselines = PlayerCharacter.Reputation
	TextMacroImperial = int8(Rand() % 6)
	CareerRegion = int16(U8At(addrCareerBackgroundIDs + uint32(RandRange(0, 9))))
	CareerBioFlag = 1
	IconCycleFrame = Rand()
	CareerBioSeed = Rand()
	if classID == 18 {
		classID = CareerNearestClass()
	}
	careerText = make([]byte, 64000)
	careerText[0] = '\n'
	copy(careerText[1:], DiskReadFile(Sprintf(S(addrStrBiographyFile), classID)))
	var background []byte
	if ask {
		background = DiskReadFile(careerBioQuestionsImage)
	}
	for q := int32(0); q < careerQuestionCount; q++ {
		if ask {
			copy(xn.Screen[:64000], background)
			xn.CursorDrawn &^= 1
		}
		careerFindQuestion(q + 1)
		if ask {
			careerWaitAnswer()
		} else {
			CareerAnswer = int16(RandRange(0, int32(CareerAnswerCount)-1))
		}
		careerApplyAnswer(q)
	}
	text := TextRSCLoad(int16(classID+4116), 0, 0)
	n := 0
	for ; n < len(text) && text[n] != 0; n++ {
		switch text[n] {
		case 253, 252:
			text[n] = 0
		case 251:
			text[n] = ' '
		}
	}
	out := make([]byte, n+1)
	copy(out, text[:n])
	DiskWriteArena2File(careerBioFile, out)
	MsgboxShowRSC(35, 1)
}

func careerSkipWord(t []byte) []byte {
	for t[0] > ' ' {
		t = t[1:]
	}
	for t[0] <= ' ' {
		t = t[1:]
	}
	return t
}

func careerFindQuestion(number int32) {
	line := careerText
	for {
		line = line[bytes.IndexByte(line, '\n')+1:]
		if isDigit(line[0]) && atoiAt(line) == number {
			careerShowQuestion(line)
			return
		}
	}
}

func careerShowQuestion(text []byte) {
	text = careerSkipWord(text)
	CareerAnswerCount = 0
	text = careerDrawLines(text, false)
	careerAnswers = text
	for !isDigit(text[0]) && text[0] != 0 {
		CareerAnswerCount++
		text = careerDrawLines(careerSkipWord(text), true)
		for !isAlnum(text[0]) && text[0] != 0 {
			text = text[bytes.IndexByte(text, '\n')+1:]
		}
	}
}

func careerDrawLines(text []byte, oneLine bool) []byte {
	box := careerRect(addrCareerAnswerBoxes, int(CareerAnswerCount))
	x, y := int32(box.X0)+21, int32(box.Y0)
	if CareerAnswerCount != 0 {
		y += 5
	}
	for {
		end := bytes.IndexByte(text, '\r')
		if CareerBioAsk {
			TextDrawColoured(string(text[:end]), int32(int16(x)), int32(int16(y)), 145, 141)
		}
		y += 10
		text = text[end+2:]
		if oneLine || text[0] != '\t' {
			return text
		}
		for text[0] == '\t' {
			text = text[1:]
		}
	}
}

func careerWaitAnswer() {
	CareerAnswer = -1
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	for CareerAnswer == -1 {
		for xn.MouseButtons == 0 {
			xn.MousePollClamped()
			xn.CursorMove(xn.MouseX, xn.MouseY)
			xn.PresentInclusive(true)
		}
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
		for a := int16(0); a < CareerAnswerCount; a++ {
			b := careerRect(addrCareerAnswerBoxes, int(a)+1)
			if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
				SoundPlay(203, PlayerObject, 100)
				CareerAnswer = a
			}
		}
	}
}

func careerApplyAnswer(q int32) {
	CareerQuestion = q
	letter := byte(CareerAnswer + 'a')
	i := bytes.Index(careerAnswers[:min(2000, len(careerAnswers))], []byte{letter, '.'})
	text := careerAnswers[i:]
	text = text[bytes.IndexByte(text, '\n')+1:]
	for text[0] == '\t' {
		text = careerAnswerEffect(text)
	}
}

// careerAnswerEffect applies one effect line of an answer; the next line.
func careerAnswerEffect(text []byte) []byte {
	for text[0] <= ' ' {
		text = text[1:]
	}
	pc := PlayerCharacter
	code := string(text[:2])
	switch {
	case code == "GP":
		pc.Gold += atoiAt(careerSkipWord(text))
	case code == "rf":
		text = text[2:]
		if f := FactionFind(int16(atoiAt(text))); f != nil {
			f.Reputation += int16(atoiAt(careerSkipWord(text)))
		}
	case code == "rr":
		Regions[CareerRegion].LegalReputation += int16(atoiAt(careerSkipWord(text)))
	case code == "MI":
		text = careerSkipWord(text) // career_effect_mi_plus/minus do nothing
	case code == "RR":
		pc.ReputationMod += int8(atoiAt(careerSkipWord(text)))
	case code == "RD":
		BioModifiers += atoiAt(careerSkipWord(text))
	case code == "MR":
		CareerResistBonus += atoiAt(careerSkipWord(text))
	case code == "TH":
		CareerToHitPenalty += atoiAt(careerSkipWord(text))
	case code == "RP":
		CareerPoisonResist += atoiAt(careerSkipWord(text))
	case code == "FT":
		CareerFatigueFlag += atoiAt(careerSkipWord(text))
	case code == "AE" || code == "AF" || code == "AO":
		kind := text[1]
		person := &Character{}
		text = careerSkipWord(text)
		if text[0] == 'F' {
			person.Flags |= 1
		} else if text[0] == 'O' {
			person.Flags |= pc.Flags&1 ^ 1
		}
		text = careerSkipWord(text)
		person.Race = uint8(atoiAt(text))
		text = careerSkipWord(text)
		number := atoiAt(text)
		person.CareerID = uint16(number)
		text = careerSkipWord(text)
		person.Level = uint8(atoiAt(text))
		class := decodeCareer(DiskReadFile(Sprintf("class%02d.cfg", number)))
		if kind == 'E' {
			copy(person.Name[:], ParseExpand(append(Bytes(addrStrBoyName, 3), 0))+"\x00")
			BioPersonAdd(person, class, 0)
		} else {
			copy(person.Name[:], ParseExpand(append(Bytes(addrStrGirlName, 3), 0))+"\x00")
			BioPersonAdd(person, class, 1)
		}
	case code == "IT":
		text = careerSkipWord(text)
		group := atoiAt(text)
		o := ObjectCreateChild(PlayerEntity, nil, itemSize)
		o.Type = 2
		o.Flags = 1
		o.Item = &Item{}
		text = careerSkipWord(text)
		ForcedMaterial = int8(atoiAt(careerSkipWord(text)) + 1)
		ItemMake(uint16(group), atoiAt(text), o.Item)
		if o.Item.Group == 3 && o.Item.Index == 18 {
			o.Item.StackCount = 1
			InvMergeArrows(PlayerEntity, o, true)
		} else {
			InvStoreItem(o)
		}
	case text[0] == '&':
		CareerBioFlag = 0
	case text[0] == '#':
		BioAnswerTexts[CareerQuestion] = atoiAt(text[1:])
	case text[0] == '!':
		BioAnswerTexts[CareerQuestion+12] = atoiAt(text[1:])
	case text[0] == '?':
		BioAnswerTexts[CareerQuestion+24] = atoiAt(text[1:])
	case isDigit(text[0]):
		n := atoiAt(text)
		if n >= 35 {
			n = 0
		}
		pc.Skills[n].Value += int16(atoiAt(careerSkipWord(text)))
	case text[0] == 'r' && isDigit(text[1]):
		pc.Reputation[atoiAt(text[1:])] += int16(atoiAt(careerSkipWord(text)))
	}
	return text[bytes.IndexByte(text, '\n')+1:]
}

// BioPersonAdd adds a person of the background to the player's records; its number, or
// 0xFFFF when the kind's eight are taken.
func BioPersonAdd(person *Character, class Career, kind int32) uint16 {
	if BioPersonCounts[kind] == careerPersonSlotsPerKind {
		return 0xFFFF
	}
	o := ObjectCreateChild(PlayerEntity, nil, careerPersonCharacterSize)
	o.Type = uint8(kind + 45)
	o.Flags |= 3
	o.Image = uint16(BioPersonCounts[kind])
	BioPersonCounts[kind]++
	c := *person
	c.Career = class
	o.Character = &c
	return o.Image
}

// CareerSlotWeight is how much a class's skill slot counts: primary 3, major 2, minor 1.
func CareerSlotWeight(slot int32) int32 {
	switch {
	case slot > 5:
		return 1
	case slot > 2:
		return 2
	}
	return 3
}

// CareerNearestClass is the standard class whose skills are most like the custom class's.
func CareerNearestClass() int32 {
	var scores [18]int32
	var classes [18]Career
	for i := range classes {
		classes[i] = decodeCareer(DiskReadFile(Sprintf("class%02d.cfg", i)))
	}
	k := int32(0)
	for i := int32(0); i < 12; i++ {
		weight := CareerSlotWeight(i)
		for j := range classes {
			k = int32(bytes.IndexByte(classes[j].Skills[:12], PlayerClass.Skills[i]))
			if k >= 0 {
				if w := CareerSlotWeight(k); w == weight {
					scores[j] += weight
				} else {
					scores[j] += 3 - abs32(w-weight)
				}
			}
		}
	}
	best := int32(0)
	k = 0
	for i, s := range scores {
		if s > best {
			best, k = s, int32(i)
		}
	}
	return k
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// InvMergeArrows adds arrows to the owner's arrow stack (making one if there is none).
func InvMergeArrows(owner, arrows *Record, deleteSource bool) {
	FoundObject = nil
	ObjectFind(owner.Children, func(o *Record) bool {
		if o.Type == 2 && o.Image2 == 998 && o.Image == 0 {
			FoundObject = o
			return true
		}
		return false
	})
	if FoundObject == nil {
		stack := ObjectCreateChild(owner, nil, itemSize)
		FoundObject = stack
		stack.Type = 2
		stack.Image2 = 998
		stack.Image = 0
		stack.Item = &Item{}
		ItemMake(3, 18, stack.Item)
		stack.Item.StackCount = arrows.Item.StackCount
		if owner == PlayerEntity {
			InvStoreItem(stack)
		}
	} else {
		count := int32(FoundObject.Item.StackCount) + int32(arrows.Item.StackCount)
		if count >= 200 {
			count = 199
		}
		FoundObject.Item.StackCount = uint8(int8(count))
	}
	if deleteSource {
		ObjectDelete(arrows)
	}
}

// CareerShowBiography shows BIO.DAT a page at a time.
func CareerShowBiography() {
	careerBioWindowShot = DiskReadFile(careerBioImage)
	CareerBioText = DiskReadFile(careerBioFile)
	CareerBioPage = 0
	SoundPlay(237, PlayerObject, 100)
	CareerBioLines = careerBioCountLines()
	CareerBioLine = 0
	for done := false; !done; {
		careerBioDraw()
		if xn.MouseButtons&1 == 0 || xn.MouseButtonsPrev&1 != 0 {
			continue
		}
		for button := 0; button < 3; button++ {
			b := careerRect(addrCareerBioButtons, button)
			if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
				switch button {
				case 0:
					CareerBioPageUp()
				case 1:
					CareerBioPageDown()
				default:
					done = true
				}
			}
		}
	}
	CareerBioText, careerBioWindowShot = nil, nil
}

func CareerBioPageDown() {
	if CareerBioLine+careerBioPageLines >= CareerBioLines {
		return
	}
	for range careerBioPageLines {
		for CareerBioText[CareerBioPage] != 0 {
			CareerBioPage++
		}
		CareerBioPage++
		CareerBioLine++
	}
}

func CareerBioPageUp() {
	if CareerBioPage == 0 {
		return
	}
	for range careerBioPageLines {
		CareerBioPage -= 2
		for CareerBioPage >= 0 && CareerBioText[CareerBioPage] != 0 {
			CareerBioPage--
		}
		CareerBioLine--
	}
	if CareerBioPage >= 0 {
		return
	}
	CareerBioPage = 0
	CareerBioLine = 0
}

func careerBioDraw() {
	copy(xn.Screen[:64000], careerBioWindowShot)
	line := CareerBioPage
	for row := int32(0); line < len(CareerBioText) && row < careerBioPageLines; row++ {
		s := xn.CStr(CareerBioText[line:])
		TextDrawColoured(s, 10, int32(int16(row*7+25)), 145, 156)
		line += len(s) + 1
	}
	CursorDrawArrow()
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	xn.PresentInclusive(true)
}

func careerBioCountLines() int32 {
	count := int32(0)
	for line := 0; line < len(CareerBioText); count++ {
		line += len(xn.CStr(CareerBioText[line:])) + 1
	}
	return count
}
