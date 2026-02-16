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
	i2cAddr      = 0x3C // Default SSD1309 I2C address
)

// OLED wraps the SSD1306 device and provides high-level rendering.
type OLED struct {
	dev ssd1306.Device
}

// white is the "on" pixel color for monochrome OLED.
var white = color.RGBA{R: 255, G: 255, B: 255, A: 255}

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
	o.dev.ClearDisplay()

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
//   Row 1: CLT + OIL temps
//   Row 2: GBX temp
//   ─── separator ───
//   Row 3: Boost pressure (mbar)
//   Row 4: Status bar
func (o *OLED) Render(data *can.VehicleData) {
	o.dev.ClearDisplay()

	// ── Row 1: Coolant + Oil (y=10) ──
	cltStr := "CLT:" + fmtI16(data.CoolantTemp) + "C"
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 0, 10, cltStr, white)

	oilStr := "OIL:" + fmtI16(data.OilTemp) + "C"
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 72, 10, oilStr, white)

	// ── Row 2: Gearbox oil temp (y=22) ──
	gbxStr := "GBX:" + fmtI16(data.GearboxOilTemp) + "C"
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 0, 22, gbxStr, white)

	// ── Separator ──
	drawHLine(&o.dev, 26)

	// ── Row 3: Boost pressure in mbar (y=40) ──
	boostStr := "BOOST:"
	if data.BoostMbar >= 0 {
		boostStr += "+" + fmtI16(data.BoostMbar)
	} else {
		boostStr += fmtI16(data.BoostMbar)
	}
	boostStr += " mbar"
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 0, 40, boostStr, white)

	// ── Separator ──
	drawHLine(&o.dev, 48)

	// ── Row 4: Status bar (y=60) ──
	statusStr := "ID:" + fmtHex(data.LastID) + " #" + fmtU32(data.MsgCount)
	tinyfont.WriteLine(&o.dev, &proggy.TinySZ8pt7b, 0, 60, statusStr, white)

	o.dev.Display()
}

// ── Helpers ──

// drawHLine draws a horizontal line across the full screen width at y.
func drawHLine(dev *ssd1306.Device, y int16) {
	for x := int16(0); x < screenWidth; x++ {
		dev.SetPixel(x, y, white)
	}
}

func fmtI16(v int16) string {
	return strconv.FormatInt(int64(v), 10)
}

func fmtU32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func fmtHex(v uint32) string {
	s := strconv.FormatUint(uint64(v), 16)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}
