package main

import (
	"errors"
	"machine"
	"time"
)

// ── MCP2515 Listen-Only access ──
//
// The TinyGo mcp2515 driver can only start the controller in Normal mode,
// where it acknowledges every frame and sends error frames, and it cannot
// tell standard remote frames from data frames. A sniffer must never drive
// the vehicle bus, so the controller is used here directly over SPI: it goes
// straight from Configuration to Listen-Only mode and is then only read.

// MCP2515 SPI instructions
const (
	mcpInstrWrite      byte = 0x02
	mcpInstrRead       byte = 0x03
	mcpInstrBitModify  byte = 0x05
	mcpInstrReadRXB0   byte = 0x90 // read RXB0 from SIDH; clears RX0IF
	mcpInstrReadRXB1   byte = 0x94 // read RXB1 from SIDH; clears RX1IF
	mcpInstrReadStatus byte = 0xA0
	mcpInstrReset      byte = 0xC0
)

// MCP2515 registers
const (
	mcpRegCANSTAT  byte = 0x0E
	mcpRegCANCTRL  byte = 0x0F
	mcpRegCNF3     byte = 0x28
	mcpRegCNF2     byte = 0x29
	mcpRegCNF1     byte = 0x2A
	mcpRegCANINTE  byte = 0x2B
	mcpRegEFLG     byte = 0x2D
	mcpRegRXB0CTRL byte = 0x60
	mcpRegRXB1CTRL byte = 0x70
)

// Acceptance filters RXF0–RXF5 and masks RXM0–RXM1; each is 4 registers
// (SIDH, SIDL, EID8, EID0) starting at these addresses.
var mcpRegAcceptance = [...]byte{0x00, 0x04, 0x08, 0x10, 0x14, 0x18, 0x20, 0x24}

// MCP2515 register and status bits
const (
	mcpModeListenOnly byte = 0x60 // CANCTRL.REQOP / CANSTAT.OPMOD
	mcpModeConfig     byte = 0x80
	mcpModeMask       byte = 0xE0
	mcpRXB0Rollover   byte = 0x04 // RXB0CTRL.BUKT: use RXB1 when RXB0 is full
	mcpRXInterrupts   byte = 0x03 // CANINTE.RX1IE | RX0IE
	mcpRXOverflow     byte = 0xC0 // EFLG.RX1OVR | RX0OVR
	mcpStatusRX0IF    byte = 0x01 // READ STATUS: frame pending in RXB0
	mcpStatusRX1IF    byte = 0x02 // READ STATUS: frame pending in RXB1
	mcpSIDLExtended   byte = 0x08 // RXBnSIDL.IDE
	mcpSIDLRemote     byte = 0x10 // RXBnSIDL.SRR: standard remote frame
	mcpDLCRemote      byte = 0x40 // RXBnDLC.RTR: extended remote frame
	mcpDLCMask        byte = 0x0F
)

const mcpModeTimeout = 200 * time.Millisecond

// Error texts are shown on the OLED: keep them within 20 characters.
var (
	errMCPNotFound = errors.New("MCP2515 not found")
	errMCPMode     = errors.New("mode change timeout")
)

// mcp2515 gives register-level access to the CAN controller. After init it
// must only be used from readCAN.
type mcp2515 struct {
	spi *machine.SPI
	cs  machine.Pin
}

// canFrame is a frame read from the MCP2515.
type canFrame struct {
	id   uint32 // 11-bit identifier
	dlc  uint8  // number of valid bytes in data
	data [8]byte
	ext  bool // extended (29-bit) frame; id holds only its upper 11 bits
	rtr  bool // remote frame; carries no data
}

// initListenOnly resets the MCP2515, applies the bit timing and puts it in
// Listen-Only mode, accepting every standard (11-bit) frame.
func (m mcp2515) initListenOnly(cnf1, cnf2, cnf3 byte) error {
	// Reset leaves the controller in Configuration mode
	m.cs.Low()
	m.spi.Transfer(mcpInstrReset)
	m.cs.High()
	time.Sleep(10 * time.Millisecond)

	if m.setMode(mcpModeConfig) != nil {
		return errMCPNotFound
	}

	m.write(mcpRegCNF1, cnf1)
	m.write(mcpRegCNF2, cnf2)
	m.write(mcpRegCNF3, cnf3)

	// Masks of 0 accept any ID; filters of 0 select standard frames
	for _, reg := range mcpRegAcceptance {
		for i := byte(0); i < 4; i++ {
			m.write(reg+i, 0)
		}
	}
	m.write(mcpRegRXB0CTRL, mcpRXB0Rollover)
	m.write(mcpRegRXB1CTRL, 0)
	m.write(mcpRegCANINTE, mcpRXInterrupts)

	return m.setMode(mcpModeListenOnly)
}

// setMode requests an operating mode and waits until the controller reports it.
func (m mcp2515) setMode(mode byte) error {
	start := time.Now()
	for {
		m.bitModify(mcpRegCANCTRL, mcpModeMask, mode)
		if m.read(mcpRegCANSTAT)&mcpModeMask == mode {
			return nil
		}
		if time.Since(start) > mcpModeTimeout {
			return errMCPMode
		}
	}
}

// receive reads the oldest pending frame into f and frees its receive buffer.
// It returns false if no frame is pending.
func (m mcp2515) receive(f *canFrame) bool {
	m.cs.Low()
	m.spi.Transfer(mcpInstrReadStatus)
	status, _ := m.spi.Transfer(0)
	m.cs.High()

	var instr byte
	switch {
	case status&mcpStatusRX0IF != 0:
		instr = mcpInstrReadRXB0
	case status&mcpStatusRX1IF != 0:
		instr = mcpInstrReadRXB1
	default:
		return false
	}

	m.cs.Low()
	m.spi.Transfer(instr)
	sidh, _ := m.spi.Transfer(0)
	sidl, _ := m.spi.Transfer(0)
	m.spi.Transfer(0) // EID8
	m.spi.Transfer(0) // EID0
	dlc, _ := m.spi.Transfer(0)

	f.id = uint32(sidh)<<3 | uint32(sidl)>>5
	f.ext = sidl&mcpSIDLExtended != 0
	if f.ext {
		f.rtr = dlc&mcpDLCRemote != 0
	} else {
		f.rtr = sidl&mcpSIDLRemote != 0
	}

	f.dlc = dlc & mcpDLCMask
	if f.dlc > 8 {
		f.dlc = 8
	}
	if f.rtr {
		f.dlc = 0 // the buffer still holds the data of an older frame
	}
	for i := uint8(0); i < f.dlc; i++ {
		f.data[i], _ = m.spi.Transfer(0)
	}
	m.cs.High() // releases the buffer (clears RXnIF)

	return true
}

// rxOverflowed reports whether the MCP2515 has dropped a frame because both
// receive buffers were full, and clears the overflow flags.
func (m mcp2515) rxOverflowed() bool {
	if m.read(mcpRegEFLG)&mcpRXOverflow == 0 {
		return false
	}
	m.bitModify(mcpRegEFLG, mcpRXOverflow, 0)
	return true
}

func (m mcp2515) read(reg byte) byte {
	m.cs.Low()
	m.spi.Transfer(mcpInstrRead)
	m.spi.Transfer(reg)
	v, _ := m.spi.Transfer(0)
	m.cs.High()
	return v
}

func (m mcp2515) write(reg, value byte) {
	m.cs.Low()
	m.spi.Transfer(mcpInstrWrite)
	m.spi.Transfer(reg)
	m.spi.Transfer(value)
	m.cs.High()
}

func (m mcp2515) bitModify(reg, mask, value byte) {
	m.cs.Low()
	m.spi.Transfer(mcpInstrBitModify)
	m.spi.Transfer(reg)
	m.spi.Transfer(mask)
	m.spi.Transfer(value)
	m.cs.High()
}
