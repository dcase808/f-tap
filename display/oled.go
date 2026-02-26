// Package display handles rendering vehicle data to a 2.42" SSD1309 OLED
// via the SSD1306-compatible TinyGo driver (128×64, I2C).
package display

import (
	"image/color"
	"machine"
	"strconv"

	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"

	"github.com/dcase808/f-tap/can"
)

const (
	screenWidth  = 128
	screenHeight = 64
	i2cAddr      = 0x3C
	fontWidth    = 6
)

// G-force visualizer parameters
const (
	gCenterX       int16   = 64
	gCenterY       int16   = 32
	gRadius        int16   = 18
	gMaxAccel      float32 = 15.0 // m/s² full scale (≈1.5g)
	smoothingAlpha float32 = 0.3  // EMA smoothing (0.3 = minimal smoothing)
)

// OLED wraps the SSD1306 device and provides high-level rendering.
type OLED struct {
	dev               ssd1306.Device
	smoothedLatAccel  float32
	smoothedLongAccel float32
}

// white is the "on" pixel color for monochrome OLED.
var white = color.RGBA{R: 255, G: 255, B: 255, A: 255}

// Icon bitmaps — each byte is one row, MSB = leftmost pixel.
var (
	// Oil drop (7px wide × 8px tall)
	iconOil = [8]byte{0x10, 0x38, 0x7C, 0xFE, 0xFE, 0xFE, 0x7C, 0x38}

	// Gear/cog (8px wide × 8px tall)
	iconGear = [8]byte{0x5A, 0xFF, 0xC3, 0xBD, 0xBD, 0xC3, 0xFF, 0x5A}

	// Thermometer (5px wide × 8px tall)
	iconThermo = [8]byte{0x20, 0x50, 0x70, 0x50, 0x70, 0xF8, 0xF8, 0x70}
)

// New creates and configures the OLED display on the given I2C bus.
func New(bus *machine.I2C) *OLED {
	dev := ssd1306.NewI2C(bus)
	dev.Configure(ssd1306.Config{
		Address: i2cAddr,
		Width:   screenWidth,
		Height:  screenHeight,
	})
	dev.ClearDisplay()

	return &OLED{dev: dev}
}

// ShowSplash displays the boot splash screen.
func (o *OLED) ShowSplash() {
	o.dev.ClearDisplay()

	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 22, 20, "F - T A P", white)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 8, 36, "BMW PT-CAN Sniffer", white)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 18, 52, "Initializing...", white)

	o.dev.Display()
}

// ShowWaiting shows the "waiting for CAN data" screen.
func (o *OLED) ShowWaiting(msgCount uint32) {
	o.dev.ClearBuffer()

	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 4, 12, "F-TAP  Listening", white)

	drawHLine(&o.dev, 16)

	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 4, 30, "Waiting for", white)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 4, 42, "PT-CAN data...", white)

	countStr := "Msgs: " + fmtU32(msgCount)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 4, 58, countStr, white)

	o.dev.Display()
}

// Render draws the main vehicle data screen.
//
// Layout (128×64):
//
//	Row 0-10:     Oil temp (left) | RPM (center) | Trans oil temp (right)
//	Row 14-50:    G-force crosshair visualizer + lateral/longitudinal labels
//	Row 52-60:    Air temp (left) | Speed (center) | Gear label (right)
func (o *OLED) Render(data *can.VehicleState) {
	o.dev.ClearBuffer()

	// ── Top-left: Oil temp (drop icon + value) ──
	drawIcon(&o.dev, 1, 1, iconOil[:], 7)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 10, 10, fmtF32(data.OilTemp)+"C", white)

	// ── Top-center: Engine RPM ──
	rpmStr := fmtF32(data.EngineSpeed) + "r"
	rpmX := int16(screenWidth/2) - int16(len(rpmStr))*fontWidth/2
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, rpmX, 10, rpmStr, white)

	// ── Top-right: Trans oil temp (gear icon + value) ──
	drawIcon(&o.dev, 94, 1, iconGear[:], 8)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 104, 10, fmtF32(data.TransOilTemp)+"C", white)

	// ── Center: G-force visualizer ──
	o.smoothAccel(data.LatAccel, data.LongAccel)
	drawGForceViz(&o.dev, o.smoothedLatAccel, o.smoothedLongAccel)
	drawGForceLabels(&o.dev, o.smoothedLatAccel, o.smoothedLongAccel)

	// ── Bottom-left: Air temp (thermometer icon + value) ──
	drawIcon(&o.dev, 1, 52, iconThermo[:], 5)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 8, 60, fmtF32(data.AirTemp)+"C", white)

	// ── Bottom-center: Vehicle speed ──
	spdStr := fmtF32(data.VehicleSpeedKph) + "kph"
	spdX := int16(screenWidth/2) - int16(len(spdStr))*fontWidth/2
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, spdX, 60, spdStr, white)

	// ── Bottom-right: Gear label ──
	gearStr := gearLabel(data.Gear)
	gearX := int16(screenWidth - len(gearStr)*fontWidth - 2)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, gearX, 60, gearStr, white)

	o.dev.Display()
}

// ── G-Force Visualizer ──

// smoothAccel applies EMA smoothing to acceleration values.
func (o *OLED) smoothAccel(latAccel, longAccel float32) {
	o.smoothedLatAccel = smoothingAlpha*latAccel + (1-smoothingAlpha)*o.smoothedLatAccel
	o.smoothedLongAccel = smoothingAlpha*longAccel + (1-smoothingAlpha)*o.smoothedLongAccel
}

// drawGForceLabels draws the X/Y acceleration labels beside the visualizer.
func drawGForceLabels(dev *ssd1306.Device, latAccel, longAccel float32) {
	rightLabelX := gCenterX + gRadius + 2
	leftEdgeX := gCenterX - gRadius - 2

	// Right side: X (lateral), left-aligned at circle_right + 2
	tinyfont.WriteLine(dev, &proggy.TinySZ8pt7b, rightLabelX, 30, "X:"+fmtF32One(latAccel), white)
	tinyfont.WriteLine(dev, &proggy.TinySZ8pt7b, rightLabelX, 40, "m/s2", white)
	// Left side: Y (longitudinal), right-aligned to circle_left - 2
	yStr := "Y:" + fmtF32One(longAccel)
	tinyfont.WriteLine(dev, &proggy.TinySZ8pt7b, leftEdgeX-int16(len(yStr))*fontWidth, 30, yStr, white)
	tinyfont.WriteLine(dev, &proggy.TinySZ8pt7b, leftEdgeX-4*fontWidth, 40, "m/s2", white)
}

// drawGForceViz draws a circle with crosshair and a moving dot
// representing current lateral and longitudinal acceleration.
func drawGForceViz(dev *ssd1306.Device, latAccel, longAccel float32) {
	// Circle outline
	drawCircle(dev, gCenterX, gCenterY, gRadius)

	// Crosshair lines
	for x := gCenterX - gRadius; x <= gCenterX+gRadius; x++ {
		dev.SetPixel(x, gCenterY, white)
	}
	for y := gCenterY - gRadius; y <= gCenterY+gRadius; y++ {
		dev.SetPixel(gCenterX, y, white)
	}

	// Map acceleration → pixel offset (lateral=X, longitudinal=Y inverted)
	dx := int16(latAccel * float32(gRadius) / gMaxAccel)
	dy := int16(-longAccel * float32(gRadius) / gMaxAccel)

	// Clamp to radius
	if dx > gRadius {
		dx = gRadius
	}
	if dx < -gRadius {
		dx = -gRadius
	}
	if dy > gRadius {
		dy = gRadius
	}
	if dy < -gRadius {
		dy = -gRadius
	}

	// Draw dot (3×3 filled square)
	dotX := gCenterX + dx
	dotY := gCenterY + dy
	for py := dotY - 1; py <= dotY+1; py++ {
		for px := dotX - 1; px <= dotX+1; px++ {
			if px >= 0 && px < screenWidth && py >= 0 && py < screenHeight {
				dev.SetPixel(px, py, white)
			}
		}
	}
}

// drawCircle draws a circle outline using the midpoint circle algorithm.
func drawCircle(dev *ssd1306.Device, cx, cy, r int16) {
	x := r
	y := int16(0)
	d := 1 - r

	for x >= y {
		dev.SetPixel(cx+x, cy+y, white)
		dev.SetPixel(cx+y, cy+x, white)
		dev.SetPixel(cx-y, cy+x, white)
		dev.SetPixel(cx-x, cy+y, white)
		dev.SetPixel(cx-x, cy-y, white)
		dev.SetPixel(cx-y, cy-x, white)
		dev.SetPixel(cx+y, cy-x, white)
		dev.SetPixel(cx+x, cy-y, white)

		y++
		if d > 0 {
			x--
			d += 2*(y-x) + 1
		} else {
			d += 2*y + 1
		}
	}
}

// ── Drawing Helpers ──

// drawIcon renders a bitmap icon. Each byte = one row, MSB = leftmost pixel.
func drawIcon(dev *ssd1306.Device, x, y int16, bitmap []byte, w int16) {
	for row := range bitmap {
		for col := int16(0); col < w; col++ {
			if bitmap[row]&(0x80>>uint(col)) != 0 {
				dev.SetPixel(x+col, y+int16(row), white)
			}
		}
	}
}

func drawHLine(dev *ssd1306.Device, y int16) {
	for x := int16(0); x < screenWidth; x++ {
		dev.SetPixel(x, y, white)
	}
}

// ── Gear Mapping ──

// gearLabel maps a raw gear value to a display label.
// BMW F-Series EGS: 0=N, 1–8=forward gears, 14=P, 15=R.
func gearLabel(raw float32) string {
	g := int32(raw + 0.5) // round to nearest int
	switch {
	case g == 0:
		return "N"
	case g >= 1 && g <= 8:
		return strconv.FormatInt(int64(g), 10)
	case g == 14:
		return "P"
	case g == 15:
		return "R"
	default:
		return "-"
	}
}

// ── Format Helpers ──

// fmtF32 formats a float32 as a rounded integer string.
func fmtF32(v float32) string {
	if v < 0 {
		return "-" + strconv.FormatInt(int64(-v+0.5), 10)
	}
	return strconv.FormatInt(int64(v+0.5), 10)
}

// fmtF32One formats a float32 with one decimal place (rounded).
func fmtF32One(v float32) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	// Round to one decimal place
	v += 0.05
	intPart := int32(v)
	fracPart := int32((v - float32(intPart)) * 10)
	return sign + strconv.FormatInt(int64(intPart), 10) + "." + strconv.FormatInt(int64(fracPart), 10)
}

func fmtU32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}
