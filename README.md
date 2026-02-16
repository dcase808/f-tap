# F-TAP

**BMW F-Series PT-CAN Sniffer** — built with [TinyGo](https://tinygo.org/) for the Raspberry Pi Pico (RP2040).

Taps into the BMW F-Series **PT-CAN** (Powertrain CAN) bus, decodes key vehicle parameters in real time, and displays them on a **2.42" OLED** screen.

## Displayed Parameters

| Parameter        | CAN ID   | Signal          | Decoding                   |
|------------------|----------|-----------------|----------------------------|
| Coolant Temp     | `0x1D0`  | `TEMP_ENG` (B0) | `raw - 48` = °C            |
| Oil Temp         | `0x1D0`  | `TEMP_EOI` (B1) | `raw - 48` = °C            |
| Boost Pressure   | `0x1D0`  | `AIP_ENG`  (B3) | `raw × 2 + 598 - 1013` mbar|
| Gearbox Oil Temp | `0x0B5`  | `Gearbox_temperature` (B7) | `raw` = °C |

> **Note:** CAN IDs sourced from [opendbc](https://github.com/commaai/opendbc) `bmw_e9x_e8x.dbc`, confirmed to work on many F-Series models. Edit [`can/ptcan.go`](can/ptcan.go) to adjust for your vehicle.

## Hardware

| Component           | Description                              |
|---------------------|------------------------------------------|
| Raspberry Pi Pico   | RP2040 microcontroller                   |
| MCP2515 Module      | SPI CAN bus controller (8 MHz crystal)   |
| 2.42" OLED          | SSD1309, 128×64, I2C (0x3C)             |

## Wiring

```
 Pico RP2040            MCP2515              2.42" OLED
 ──────────           ─────────             ──────────
 GP18 (SCK)  ───────▶ SCK
 GP19 (MOSI) ───────▶ SI
 GP16 (MISO) ◀─────── SO
 GP17 (CS)   ───────▶ CS
 3V3         ───────▶ VCC
 GND         ───────▶ GND

 GP14 (SDA)  ──────────────────────────────▶ SDA
 GP15 (SCL)  ──────────────────────────────▶ SCL
 3V3         ──────────────────────────────▶ VCC
 GND         ──────────────────────────────▶ GND
```

## Build & Flash

**Prerequisites:** [TinyGo](https://tinygo.org/getting-started/install/) installed.

```bash
# Build UF2 firmware
make build

# Flash directly to Pico (hold BOOTSEL, then plug USB)
make flash

# Serial monitor (115200 baud)
make monitor
```

Or manually:
```bash
tinygo flash -target=pico .
```

## PT-CAN Connection

> ⚠️ **WARNING:** Only connect in **listen-only mode**. Incorrect CAN bus connections can interfere with vehicle systems. Always tap — never inject.

BMW F-Series PT-CAN runs at **500 kbps**. The PT-CAN bus is accessible through:
- **OBD-II port** pins 6 (CAN-H) and 14 (CAN-L)  
- Direct connection at the DME/DDE or other PT-CAN ECUs

Connect MCP2515 **CAN-H** and **CAN-L** to the PT-CAN bus. A 120Ω termination resistor may be needed depending on your tap point.

## Project Structure

```
f-tap/
├── main.go           # Entry point, hardware init, main loop
├── can/
│   └── ptcan.go      # BMW PT-CAN message IDs & decoder
├── display/
│   └── oled.go       # OLED rendering (SSD1306/SSD1309)
├── go.mod
├── Makefile
└── README.md
```

MIT
