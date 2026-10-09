package main

import (
	_ "embed"
	"math"

	"github.com/egoist/mygo/ui"
)

// Warm paper, and the two things allowed to leave it: the history popover and
// the row under the cursor during a drag. Colour is rationed to one job — red
// means priority.
var (
	// Warm — R and G above B — because a cool grey reads as clinical at this size.
	colorGround = ui.Hex("#FBFAF7")
	// The only white in the window: whatever floats above the paper.
	colorRaised   = ui.Hex("#FFFFFF")
	colorHairline = ui.Hex("#EBE7E0")

	colorInk      = ui.Hex("#1F1D1A")
	colorInkSoft  = ui.Hex("#8B857A")
	colorInkFaint = ui.Hex("#ADA79B")

	// Priority, and the confirm step on destructive actions.
	colorPriority = ui.Hex("#D92D20")

	// The text fields' caret and selection: ink and a pale blue, rather than
	// the theme's accent blue.
	colorCaret     = ui.Hex("#0A0A0A")
	colorSelection = ui.RGBA(0x55, 0xA0, 0xFC, 0.3)
)

// surface holds the hover tints of one plane. A tint that reads on the warm
// ground disappears on white, so the paper and the popover cannot share one.
// rowHover also paints the hover tray, which slides over the title and has to
// hide it.
type surface struct {
	rowHover, actionBg, actionBgHover, actionBgActive ui.Color
}

var (
	paper = surface{
		rowHover:       ui.Hex("#F3F0EA"),
		actionBg:       ui.Hex("#E9E5DD"),
		actionBgHover:  ui.Hex("#DED9CF"),
		actionBgActive: ui.Hex("#D1CBBF"),
	}
	popover = surface{
		rowHover:       ui.Hex("#F6F4F0"),
		actionBg:       ui.Hex("#EDEAE4"),
		actionBgHover:  ui.Hex("#E2DED6"),
		actionBgActive: ui.Hex("#D6D1C7"),
	}
)

// trayFade is transparent at the tray's left edge and solid by its first
// button, so a long title dissolves under it instead of being cut off. Both
// stops are the same colour at different alpha: fading to a generic
// transparent leaves a grey smear down the middle of the ramp.
func trayFade(s surface) ui.LinearGradient {
	return ui.LinearGradient{From: s.rowHover.Alpha(0), To: s.rowHover, Angle: 90, End: 0.25}
}

func popoverShadow(e ui.Element) ui.Element {
	return e.Shadow(0, 2, 5, 0, ui.RGBA(0, 0, 0, 0x17/255.)).
		Shadow(0, 10, 26, -6, ui.RGBA(0, 0, 0, 0x24/255.))
}

func easeOutQuint(t float32) float32 {
	return 1 - float32(math.Pow(float64(1-t), 5))
}

var (
	//go:embed icons/pin.svg
	pinSVG []byte
	//go:embed icons/pin-filled.svg
	pinFilledSVG []byte
	//go:embed icons/history.svg
	historySVG []byte
	//go:embed icons/plus.svg
	plusSVG []byte
	//go:embed icons/trash.svg
	trashSVG []byte
	//go:embed icons/rotate-ccw.svg
	rotateSVG []byte
	//go:embed icons/grip.svg
	gripSVG []byte
	//go:embed icons/flame.svg
	flameSVG []byte
	//go:embed icons/flame-filled.svg
	flameFilledSVG []byte
	//go:embed icons/check.svg
	checkSVG []byte

	iconPin         = ui.MustParseSVG(pinSVG)
	iconPinFilled   = ui.MustParseSVG(pinFilledSVG)
	iconHistory     = ui.MustParseSVG(historySVG)
	iconPlus        = ui.MustParseSVG(plusSVG)
	iconTrash       = ui.MustParseSVG(trashSVG)
	iconRotate      = ui.MustParseSVG(rotateSVG)
	iconGrip        = ui.MustParseSVG(gripSVG)
	iconFlame       = ui.MustParseSVG(flameSVG)
	iconFlameFilled = ui.MustParseSVG(flameFilledSVG)
	iconCheck       = ui.MustParseSVG(checkSVG)
)
