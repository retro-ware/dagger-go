package game

import "github.com/retro-ware/dagger-go/internal/xn"

// Scalar globals whose initial values FALL.EXE's data segment sets (non-zero at start).

func init() {
	tableLoaders = append(tableLoaders, func() {
		CfgTextureMem = int32(U32At(0x185208))
		CfgHelmet = int32(U32At(0x153404))
		GuardsTimer = int32(U32At(0x178A14))
		GuardTeam = U8At(0x17B6CD)
		LastUnloadedLocation = int32(U32At(0x187F28))
		nextRecordID = int32(U32At(0x187FDC))
		PlayerEnvironment = U8At(0x1789FA)
		PlayerMomentum = int32(U32At(0x187CA4))
		PlayerSpeed = int16(U16At(0x178A01))
		PlayerSpeedBonus = int32(U32At(0x187CA9))
		SteerTurnSpeedMax = int32(U32At(0x1788CF))
		ViewWidth = int16(U16At(0x1789FD))
		ViewHeight = int16(U16At(0x1789FF))
		xn.CamCentreX = int32(int16(U16At(0x0CEA30)))
		xn.CamCentreY = int32(int16(U16At(0x0CEA34)))
		xn.MouseX = int16(U16At(0x12AC04))
		xn.MouseY = int16(U16At(0x12AC06))
		xn.MouseXMax = int16(U16At(0x12AF36))
		xn.MouseYMax = int16(U16At(0x12AF3A))
	})
}
