package game

import (
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// bank.c: the bank window (gold, letters of credit, loans, houses and ships) and the
// loan collectors.

const (
	addrBankButtons          = 0x186E24
	addrLoanCollectorMonster = 0x17A028
	strBank00i0Img           = 0x175C90
	strBank01i0Img           = 0x175C9D
	strBank01i1Img           = 0x175CAA
	strBank01i2Img           = 0x175CB7
	strAt                    = 0x175CCB
	strCannotCarryGold       = 0x175CD0
	strPriceGold             = 0x175CEF
)

// bankHouse is an entry of bank_houses_for_sale.
type bankHouse struct {
	Block    *Block
	Building *Building
	Price    int32
	ID       uint32
	SavedYaw int32
}

// bankShip is an entry of bank_ships_for_sale.
type bankShip struct {
	Model BlockModel
	ID    uint32
	Price int32
}

var (
	BankHousesForSale       [64]bankHouse
	BankHouseCount          uint8
	BankShipsForSale        [2]bankShip
	BankShipCount           uint8
	BankBoughtHouse         *Record
	BankBoughtHouseBuilding *Building
	BankAccountOpen         *BankAccount // bank_account: the current region's account
	BankScreen              uint8
	BankSelected            uint8
	BankListTop             int32
	BankPreviewDistance     int32
	BankAmountText          [16]byte
	bankSavedScreen         []byte
	bankScreenImage         []byte // screen_image_1
	bankListImage           []byte
	bankHouseImage          []byte
	bankButtons             []menuButton
)

// BankOpen opens the bank window (opening) and says whether it is open.
func BankOpen(opening int32) bool {
	if GameMode == 8 && GameModeUnderPopup == 15 {
		return true
	}
	if opening != 0 {
		waitMouseUp()
		BankBoughtHouse = nil
		WindowImage = DiskReadFile(S(strBank00i0Img))
		bankScreenImage = DiskReadFile(S(strBank01i0Img))
		bankListImage = DiskReadFile(S(strBank01i1Img))
		bankHouseImage = DiskReadFile(S(strBank01i2Img))
		xn.CamSetViewWindow(51, 45, 216, 72)
		ScreenActive = 1
		GameMode = 15
		WorldRunning = 0
		BankHouseCount, BankShipCount, BankScreen, BankSelected = 0, 0, 0, 0
		BankListTop = 0
		BankPreviewDistance = 1500
		bankSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
		BankAccountOpen = &BankAccounts.BankAccounts[uint8(CurrentRegion)]
		ObjectForeach(LocationObject, BankAddHouseForSale)
		bankInitShips()
	}
	return GameMode == 15
}

func bankClose() {
	waitEscUp()
	if GameSettings.ViewFlags&1 != 0 {
		xn.CamSetViewWindow(160, 100, 160, 100)
	} else {
		xn.CamSetViewWindow(160, 77, 160, 77)
	}
	bankSavedScreen, WindowImage, bankScreenImage, bankListImage, bankHouseImage = nil, nil, nil, nil, nil
	TextRSCBuffer = ""
	bankRestoreHouses()
	GameMode = 0
	ScreenActive = 0
	WorldRunning = 1
}

// BankFrame runs the bank window a frame.
func BankFrame() {
	if !BankOpen(0) {
		return
	}
	if bankButtons == nil {
		bankButtons = menuButtons(addrBankButtons,
			bankDepositGold, bankWithdrawGold, bankDepositLettersOfCredit, bankWithdrawLetterOfCredit,
			bankRepayLoan, bankBorrow, bankBuyHouse, bankSellHouse, bankBuyShip, bankSellShip, bankClose,
			bankHouseListClick, bankHouseBought, bankListExit, bankHouseListUp, bankHouseListDown,
			bankShipListClick, bankShipBought, bankListExit)
	}
	bankDraw()
	if xn.KeyDown[scanEsc] != 0 {
		bankClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	var first, last int
	switch BankScreen {
	case 0:
		first, last = 0, 11
	case 1:
		first, last = 11, 16
	case 2:
		first, last = 16, 19
	}
	for _, b := range bankButtons[first:last] {
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			SoundPlay(203, PlayerObject, 100)
			b.Handler()
		}
	}
}

// BankAddHouseForSale lists a building marker of a house as for sale (object_foreach).
func BankAddHouseForSale(o *Record) {
	if o.Type != 43 || o.Children == nil || BankHouseCount == 20 {
		return
	}
	b := &CurrentLocation.Buildings[o.Image]
	if o.ID != b.ID || b.Type != 1 {
		return
	}
	k := o.Block
	h := &BankHousesForSale[BankHouseCount]
	h.Block, h.Building, h.ID = k, b, o.ID
	h.SavedYaw = k.Models[0].Yaw
	maxRadius := int32(0)
	for i := 0; i < int(k.ModelCount); i++ {
		m := &k.Models[i]
		m.Handle.Model = ModelGet(int32(m.ID), int32(m.Variant), modelClimateVariant())
		if m.Handle.Model != nil && m.Handle.Model.Radius > maxRadius {
			maxRadius = m.Handle.Model.Radius
		}
	}
	h.Price = maxRadius * 5
	BankHouseCount++
}

func bankInitShips() {
	s := &BankShipsForSale
	s[0].Model.ID, s[0].Model.Variant = 415, 6
	s[0].Model.Handle.Model = ModelGet(415, 6, modelClimateVariant())
	s[0].ID, s[0].Price = 65011713, 100000
	// the second ship is listed as variant 11 but loads variant 8
	s[1].Model.ID, s[1].Model.Variant = 415, 11
	s[1].Model.Handle.Model = ModelGet(415, 8, modelClimateVariant())
	s[1].ID, s[1].Price = 65077249, 200000
	BankShipCount = 2
}

func bankDepositGold() {
	amount := bankInputAmount()
	if amount < 1 {
		return
	}
	if uint32(PlayerCharacter.Gold) < uint32(amount) {
		MsgboxShowRSC(454, 1)
		return
	}
	BankAccountOpen.Balance += amount
	PlayerCharacter.Gold -= amount
}

func bankWithdrawGold() {
	a := BankAccountOpen
	if a.Balance <= 0 {
		return
	}
	amount := bankInputAmount()
	if amount < 1 {
		return
	}
	if a.Balance < amount {
		if !bankConfirm(290) {
			return
		}
		amount = a.Balance
	}
	if GoldCanCarry(amount) {
		a.Balance -= amount
		PlayerCharacter.Gold += amount
		return
	}
	MsgboxShowString(S(strCannotCarryGold), 1)
}

func bankDepositLetter(o *Record) {
	if o.Type != 2 {
		return
	}
	item := o.Item
	if item.Group != 27 || item.Index != 2 {
		return
	}
	BankAccountOpen.Balance += int32(item.Value)
	ObjectDelete(o)
}

func bankDepositLettersOfCredit() {
	if !bankConfirm(291) {
		return
	}
	ObjectForeach(PlayerEntity.Children, bankDepositLetter)
}

func bankWithdrawLetterOfCredit() {
	a := BankAccountOpen
	if a.Balance <= 0 {
		return
	}
	amount := bankInputAmount()
	if amount < 1 {
		return
	}
	if amount < 100 {
		MsgboxShowRSC(293, 1)
		return
	}
	fee := amount/100 + 1
	if fee+amount > a.Balance {
		MsgboxShowRSC(292, 1)
		return
	}
	a.Balance -= fee + amount
	letter := ObjectCreateChild(InventoryContainers[2], nil, 107)
	letter.Type = 2
	letter.Flags = 1
	ItemMake(27, 2, letter.Item)
	letter.Item.Value = uint32(amount)
}

func bankRepayLoan() {
	a := BankAccountOpen
	if a.LoanOwed == 0 {
		return
	}
	amount := bankInputAmount()
	if amount < 1 {
		return
	}
	if a.Balance+GoldTotal() < amount {
		MsgboxShowRSC(454, 1)
		return
	}
	if amount > a.LoanOwed {
		amount = a.LoanOwed
		MsgboxShowRSC(294, 1)
	}
	a.Balance -= amount
	a.LoanOwed -= amount
	if a.Balance < 0 {
		GoldSpend(-a.Balance)
		a.Balance = 0
	}
	if a.LoanOwed == 0 {
		a.LoanDue = 0
	}
}

func bankBorrow() {
	a := BankAccountOpen
	if a.Flags&1 != 0 {
		MsgboxShowRSC(288, 1)
		return
	}
	if a.LoanDue != 0 {
		MsgboxShowRSC(289, 1)
		return
	}
	amount := bankInputAmount()
	if amount < 1 {
		return
	}
	if amount < 100 {
		MsgboxShowRSC(296, 1)
		return
	}
	if int32(PlayerCharacter.Level)*50000 < amount {
		MsgboxShowRSC(295, 1)
		return
	}
	a.LoanDue = uint32(GameMinutes) + 518400
	a.LoanOwed = amount + amount*10/100
	a.Balance += amount
}

func bankBuyHouse() {
	if BankHouseCount == 0 {
		MsgboxShowRSC(287, 1)
		return
	}
	if PlayerCharacter.House != 0 {
		MsgboxShowRSC(286, 1)
		return
	}
	BankScreen = 1
	BankSelected = 0
}

func bankSellHouse() {
	if PlayerCharacter.House == 0 {
		return
	}
	TradePrice = BankHousePrice - BankHousePrice*15/100
	if !bankConfirm(298) {
		return
	}
	BankAccountOpen.Balance += TradePrice
	BankHousePrice = 0
	PlayerCharacter.House = 0
}

func bankBuyShip() {
	if PlayerCharacter.ShipOwned != 0 {
		MsgboxShowRSC(284, 1)
		return
	}
	if LocationExterior[0x2F] == 0 { // location_is_port
		MsgboxShowRSC(285, 1)
		return
	}
	BankScreen = 2
	BankSelected = 0
}

func bankSellShip() {
	if PlayerCharacter.ShipOwned == 0 {
		return
	}
	TradePrice = BankShipPrice - BankShipPrice*15/100
	if !bankConfirm(299) {
		return
	}
	BankAccountOpen.Balance += TradePrice
	BankShipPrice = 0
	PlayerCharacter.ShipOwned = 0
}

// bankDrawPreview draws the selected house or ship turning in the window's view.
func bankDrawPreview(models []BlockModel) {
	xn.CamFarZ = 1048576
	xn.ShadeSetFog(-1)
	xn.LightAmbient = 16128
	xn.CamX = 0
	xn.CamY = -100
	if xn.KeyDown[scanMinus] != 0 {
		BankPreviewDistance -= 8
	} else if xn.KeyDown[scanEquals] != 0 {
		BankPreviewDistance += 8
	}
	BankPreviewDistance = max(100, min(BankPreviewDistance, 2000))
	xn.CamZ = -BankPreviewDistance
	xn.CamPitch, xn.CamYaw, xn.CamRoll = 0, 0, 0
	xn.TexCacheBeginFrame()
	xn.RenderBeginFrame()
	xn.LightReset()
	xn.CamRotation = xn.MatFromAngles(xn.CamPitch, xn.CamYaw, xn.CamRoll)
	xn.CamViewMatrix = xn.CamScaleMatrix(&xn.CamRotation)
	// the origin is the last real model (C left it unset with none)
	var ox, oy, oz int32
	for i := range models {
		if m := &models[i]; m.ID <= 10 || m.ID == 415 {
			ox, oy, oz = m.X, m.Y, m.Z
		}
	}
	for i := range models {
		m := &models[i]
		m.X -= ox
		m.Y -= oy
		m.Z -= oz
	}
	for i := range models {
		m := &models[i]
		m.Handle.Model = ModelGet(int32(m.ID), int32(m.Variant), modelClimateVariant())
		if m.Handle.Model != nil {
			m.Yaw = int32(xn.BIOSTicks()&2047) << 4 & 2047
			blockModelSubmit(m)
			break
		}
	}
	if xn.RenderFrame(2) != 0 || xn.TexCacheFull {
		xn.TexCacheFull = false
		xn.TexCacheFlush()
	}
	for i := range models {
		m := &models[i]
		m.X += ox
		m.Y += oy
		m.Z += oz
	}
}

// blockModelSubmit submits a block model: C's handle shared the model's position and yaw.
func blockModelSubmit(m *BlockModel) {
	m.Handle.X, m.Handle.Y, m.Handle.Z, m.Handle.Yaw = m.X, m.Y, m.Z, m.Yaw
	xn.ModelSubmit(&m.Handle, 0)
}

func bankArrowImage(on bool) []byte {
	if on {
		return bankListImage
	}
	return bankHouseImage
}

func bankDrawHouseList() {
	for row := BankListTop; row < BankListTop+11; row++ {
		colour := uint8(146)
		if int32(BankSelected) == row {
			colour = 246
		}
		TextDrawColoured(Sprintf(SP(strPriceGold), BankHousesForSale[row].Price), 52, (row-BankListTop)*7+38, colour, 156)
	}
	xn.DrawImageTransparent(153, 38, 9, 20, bankArrowImage(BankListTop != 0))
	xn.DrawImageTransparent(153, 102, 9, 16, bankArrowImage(BankListTop+11 < int32(BankHouseCount))[576:])
}

func bankDrawShipList() {
	for row := 0; row < int(BankShipCount); row++ {
		colour := uint8(146)
		if int(BankSelected) == row {
			colour = 246
		}
		TextDrawColoured(strconv.Itoa(int(BankShipsForSale[row].Price)), 52, int32(row)*7+38, colour, 156)
	}
}

func bankHouseListClick() {
	row := BankListTop + (int32(xn.MouseY)-38)/7
	if int32(BankHouseCount) <= row {
		return
	}
	BankSelected = uint8(row)
}

func bankHouseListUp() {
	if BankListTop != 0 {
		BankListTop--
	}
}

func bankHouseListDown() {
	if BankHouseCount <= 11 || int32(BankHouseCount)-11 <= BankListTop {
		return
	}
	BankListTop++
}

func bankShipListClick() {
	row := (int32(xn.MouseY) - 38) / 7
	if int32(BankShipCount) <= row {
		return
	}
	BankSelected = uint8(row)
}

func bankHouseBought() {
	h := &BankHousesForSale[BankSelected]
	if BankAccountOpen.Balance < h.Price {
		MsgboxShowRSC(454, 1)
		return
	}
	BankHousePrice = h.Price
	BankAccountOpen.Balance -= BankHousePrice
	PlayerCharacter.House = int32(h.ID)
	BankBoughtHouse = blockRecord(h.Block)
	BankBoughtHouseBuilding = h.Building
	MsgboxShowRSC(282, 1)
	SavedRegionName = RegionName(CurrentRegion)
	SavedLocationName = CString(CurrentLocation.Name[:])
	BankScreen = 0
}

// blockRecord is RECORD_FROM_DATA of a building marker's block: the record holding it.
func blockRecord(k *Block) *Record {
	var found *Record
	ObjectForeachUntil(LocationObject, func(o *Record) bool {
		if o.Block == k {
			found = o
			return true
		}
		return false
	})
	return found
}

func bankShipBought() {
	s := &BankShipsForSale[BankSelected]
	if BankAccountOpen.Balance < s.Price {
		MsgboxShowRSC(454, 1)
		return
	}
	BankAccountOpen.Balance -= s.Price
	BankShipPrice = s.Price
	PlayerCharacter.ShipOwned = int32(s.ID)
	MsgboxShowRSC(283, 1)
	BankScreen = 0
}

func bankListExit() { BankScreen = 0 }

func bankConfirm(textID int32) bool {
	MsgboxYesNoRSC(textID)
	for {
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		bankDraw()
		MsgboxUpdate()
		CursorDrawArrow()
		xn.PresentInclusive(true)
		switch PopupAnswer {
		case 1:
			return true
		case 2:
			return false
		}
	}
}

func bankInputAmount() int32 {
	xn.TextColour = 146
	BankAmountText = [16]byte{}
	TextCursorX = 157
	TextCursorY = 152
	InpstrBeginText(BankAmountText[:], 10)
	xn.KbdFlush()
	xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
	for done := int32(0); done == 0; {
		bankDraw()
		done = InpstrUpdate()
		xn.PresentInclusive(true)
	}
	return Atoi(xn.CStr(BankAmountText[:]))
}

func bankRestoreHouses() {
	for i := 0; i < int(BankHouseCount); i++ {
		h := &BankHousesForSale[i]
		h.Block.Models[0].Yaw = h.SavedYaw
	}
}

// bankDraw draws the bank window (bank_draw).
func bankDraw() {
	copy(xn.Screen[:64000], bankSavedScreen)
	im := parseImg(WindowImage)
	xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
	a := BankAccountOpen
	switch BankScreen {
	case 0:
		TextDrawColoured(strconv.Itoa(int(a.Balance)), 197, 19, 145, 156)
		TextDrawColoured(strconv.Itoa(int(GoldTotal())), 203, 29, 145, 156)
		if a.LoanDue != 0 {
			TextDrawColoured(strconv.Itoa(int(a.LoanOwed)), 143, 39, 145, 156)
			saved := GameMinutes
			GameMinutes = a.LoanDue
			InpstrSavedText = ParseExpand(append(Bytes(strAt, 4), 0))
			TextDrawColoured(InpstrSavedText, 119, 49, 145, 156)
			GameMinutes = saved
		}
	case 1:
		im := parseImg(bankScreenImage)
		xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
		bankDrawHouseList()
		k := BankHousesForSale[BankSelected].Block
		bankDrawPreview(k.Models[:k.ModelCount])
	case 2:
		im := parseImg(bankScreenImage)
		xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
		bankDrawShipList()
		s := &BankShipsForSale[BankSelected]
		models := []BlockModel{s.Model}
		bankDrawPreview(models)
		s.Model = models[0]
	}
}

// LoanCollectorsUpdate sends collectors after an overdue loan now and then.
func LoanCollectorsUpdate() {
	if PlayerEnvironment == 3 || LoanCollectorsNext > uint32(GameMinutes) {
		return
	}
	LoanCollectorsNext = GameMinutes + uint32(RandRange(1400, 1700))
	// C never advanced its account pointer: all 62 checks look at the first region's loan
	a := &BankAccounts.BankAccounts[0]
	for i := 0; i < 62; i++ {
		if a.LoanDue != 0 && a.LoanDue < uint32(GameMinutes) {
			loanSpawnCollectors()
		}
	}
}

func loanSpawnCollectors() {
	n := RandRange(2, 5)
	for i := int32(0); i < n; i++ {
		EncounterSpawnHidden(int32(U8At(addrLoanCollectorMonster + uint32(Rand()&3))))
	}
}
