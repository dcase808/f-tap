// F-TAP — BMW F-Series PT-CAN Sniffer
//
// Taps into BMW F-Series PT-CAN bus via MCP2515 (SPI), decodes engine/drivetrain
// parameters, and displays them on a 2.42" SSD1309 OLED (I2C).
//
// Target: Raspberry Pi Pico (RP2040)
// Build:  tinygo flash -target=pico .
package main

import (
	"machine"
	"time"

	"tinygo.org/x/drivers/mcp2515"

	"github.com/dcase808/f-tap/can"
	"github.com/dcase808/f-tap/display"
)

// ── Pin Configuration ──
// MCP2515 (SPI0)
const (
	pinSCK = machine.GP18 // SPI0 SCK
	pinTX  = machine.GP19 // SPI0 TX  (MOSI)
	pinRX  = machine.GP16 // SPI0 RX  (MISO)
	pinCS  = machine.GP17 // SPI0 CS  (directly controlled by driver)
)

// OLED (I2C1)
const (
	pinSDA = machine.GP14 // I2C1 SDA
	pinSCL = machine.GP15 // I2C1 SCL
)

// CAN Bus configuration
const (
	canSpeed = mcp2515.CAN500kBps // BMW PT-CAN = 500 kbps
	canClock = mcp2515.Clock8MHz  // MCP2515 crystal oscillator frequency
)

// Display refresh interval
const displayRefreshInterval = 50 * time.Millisecond

func main() {
	// ── LED heartbeat ──
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.High()

	// Small delay for hardware power-up
	time.Sleep(100 * time.Millisecond)

	// ── Initialize I2C for OLED ──
	i2c := machine.I2C1
	err := i2c.Configure(machine.I2CConfig{
		SDA:       pinSDA,
		SCL:       pinSCL,
		Frequency: 400 * machine.KHz, // 400kHz fast mode
	})
	if err != nil {
		// If I2C fails, blink LED rapidly as error indicator
		blinkError(led)
	}

	// ── Initialize OLED display ──
	oled := display.New(i2c)
	oled.ShowSplash()
	time.Sleep(1500 * time.Millisecond)

	// ── Initialize SPI for MCP2515 ──
	spi := machine.SPI0
	err = spi.Configure(machine.SPIConfig{
		SCK:       pinSCK,
		SDO:       pinTX,
		SDI:       pinRX,
		Frequency: 1 * machine.MHz,
		Mode:      0,
	})
	if err != nil {
		blinkError(led)
	}

	// ── Initialize MCP2515 CAN controller ──
	cs := pinCS
	cs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	cs.High()

	canDev := mcp2515.New(spi, cs)
	canDev.Configure()
	err = canDev.Begin(canSpeed, canClock)
	if err != nil {
		blinkError(led)
	}

	// ── Shared vehicle state (RWMutex embedded in VehicleData) ──
	var vehicleData can.VehicleData

	// ── Launch goroutines ──
	go readCAN(canDev, &vehicleData)
	go renderDisplay(oled, &vehicleData)

	// ── Main goroutine: LED heartbeat ──
	for {
		led.Low()
		time.Sleep(displayRefreshInterval)
		led.High()
		time.Sleep(displayRefreshInterval)
	}
}

// readCAN continuously polls the MCP2515 for incoming CAN frames
// and updates vehicleData. ParseMessage acquires a write lock internally.
func readCAN(canDev *mcp2515.Device, data *can.VehicleData) {
	for {
		if canDev.Received() {
			msg, err := canDev.Rx()
			if err == nil && msg != nil {
				data.ParseMessage(msg.ID, msg.Data)
			}
		}
	}
}

// renderDisplay redraws the OLED at a fixed interval.
// It takes a read-lock snapshot of vehicleData so CAN reads aren't
// blocked during the slow I2C transfer.
func renderDisplay(oled *display.OLED, data *can.VehicleData) {
	for {
		time.Sleep(displayRefreshInterval)

		snap := data.Snapshot()
		if snap.Initialized {
			oled.Render(&snap)
		} else {
			oled.ShowWaiting(snap.MsgCount)
		}
	}
}

// blinkError blinks the LED rapidly to indicate an initialization error.
// This blocks forever — the device must be reset after fixing the issue.
func blinkError(led machine.Pin) {
	for {
		led.Low()
		time.Sleep(100 * time.Millisecond)
		led.High()
		time.Sleep(100 * time.Millisecond)
	}
}
