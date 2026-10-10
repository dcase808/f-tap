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

	"github.com/dcase808/f-tap/can"
	"github.com/dcase808/f-tap/display"
)

// ── Pin Configuration ──
// MCP2515 (SPI0)
const (
	pinSCK = machine.GP18 // SPI0 SCK
	pinTX  = machine.GP19 // SPI0 TX  (MOSI)
	pinRX  = machine.GP16 // SPI0 RX  (MISO)
	pinCS  = machine.GP17 // SPI0 CS  (driven directly, see mcp2515.go)
)

// OLED (I2C1)
const (
	pinSDA = machine.GP14 // I2C1 SDA
	pinSCL = machine.GP15 // I2C1 SCL
)

// CAN Bus configuration: MCP2515 bit timing for 500 kbps (BMW PT-CAN) with
// an 8 MHz crystal. For a 16 MHz crystal use 0x00, 0xF0, 0x86.
const (
	canCNF1 byte = 0x00
	canCNF2 byte = 0x90
	canCNF3 byte = 0x02
)

// Timing intervals
const (
	displayRefreshInterval = 50 * time.Millisecond
	heartbeatInterval      = 500 * time.Millisecond
	canPollInterval        = 200 * time.Microsecond // idle sleep between MCP2515 polls
	canCheckInterval       = 100 * time.Millisecond // overflow and bus silence checks
	busTimeout             = 2 * time.Second        // bus silent this long → data is stale
)

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
		oled.ShowError("SPI init failed", "Check SPI pins")
		blinkError(led)
	}

	// ── Initialize MCP2515 CAN controller (Listen-Only, see mcp2515.go) ──
	cs := pinCS
	cs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	cs.High()

	canDev := mcp2515{spi: spi, cs: cs}
	err = canDev.initListenOnly(canCNF1, canCNF2, canCNF3)
	if err != nil {
		oled.ShowError("CAN init failed", err.Error())
		blinkError(led)
	}

	// ── Shared vehicle state (RWMutex embedded in VehicleData) ──
	var vehicleData can.VehicleData

	// ── Launch goroutines ──
	go readCAN(canDev, &vehicleData)
	go renderDisplay(oled, &vehicleData)

	// ── Main goroutine: LED heartbeat + bus statistics on serial ──
	for {
		led.Low()
		time.Sleep(heartbeatInterval)
		led.High()
		time.Sleep(heartbeatInterval)

		snap := vehicleData.Snapshot()
		println("msgs:", snap.MsgCount, "rx errors:", snap.RxErrors, "last id:", snap.LastID)
	}
}

// readCAN continuously polls the MCP2515 for incoming CAN frames
// and updates vehicleData. ParseMessage acquires a write lock internally.
// Frames the MCP2515 had to drop are counted as receive errors, and the data
// is invalidated once the bus has been silent for busTimeout.
//
// NOTE: If the MCP2515 INT pin is wired to a GPIO, this could be replaced
// with interrupt-driven reads (machine.PinInterrupt) to reduce CPU usage
// and improve latency. With polling, the idle sleep has to stay well below
// the time two frames take to arrive (~450µs at 500 kbps), because the
// MCP2515 buffers only two.
func readCAN(canDev mcp2515, data *can.VehicleData) {
	var frame canFrame
	lastRx := time.Now()
	lastCheck := lastRx

	for {
		now := time.Now()

		if canDev.receive(&frame) {
			lastRx = now
			// PT-CAN signals are 11-bit data frames: skip extended and remote frames
			if !frame.ext && !frame.rtr {
				data.ParseMessage(frame.id, frame.data[:frame.dlc])
			}
		} else {
			time.Sleep(canPollInterval)
		}

		if now.Sub(lastCheck) >= canCheckInterval {
			lastCheck = now
			if canDev.rxOverflowed() {
				data.IncrementRxErrors()
			}
			if now.Sub(lastRx) >= busTimeout {
				data.Invalidate()
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
		if snap.Initialized() {
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
