# F-TAP

**BMW F-Series PT-CAN Sniffer** — built with [TinyGo](https://tinygo.org/) for the Raspberry Pi Pico (RP2040).

Taps into the BMW F-Series **PT-CAN** (Powertrain CAN) bus, decodes key vehicle parameters in real time, and displays them on a **2.42" OLED** screen.

## Decoded Signals

| Signal | CAN ID | Bits | Decoding | Unit |
|--------|--------|------|----------|------|
| Throttle_Position | `0x0D9` (217) | 16:12 | `raw × 0.025` | % |
| Engine_Speed | `0x0F3` (243) | 12:12 | `raw × 10` | rpm |
| Brake_Position | `0x173` (371) | 56:6 | `raw × 1` | — |
| Brake_Light | `0x173` (371) | 3:2 | `raw × 0.5` (Motorola) | on/off |
| Longitudinal_Accel | `0x199` (409) | 16:16 | `raw × 0.002 − 65` | m/s² |
| Lateral_Accel | `0x19A` (410) | 16:16 | `raw × 0.002 − 65` | m/s² |
| Vehicle_Speed_kph | `0x1A1` (417) | 16:16 | `raw × 0.015625` | km/h |
| Vehicle_Speed_mph | `0x1A1` (417) | 16:16 | `raw × 0.009703125` | mph |
| Battery_Current | `0x1BA` (442) | 24:16 | `raw × 0.02 − 200` | — |
| Wheel_Speed_RL | `0x254` (596) | 0:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_RR | `0x254` (596) | 16:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_FL | `0x254` (596) | 32:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_FR | `0x254` (596) | 48:15 | `raw × 0.015625` (Signed) | km/h |
| Battery_Voltage | `0x281` (641) | 0:12 | `raw × 15` | mV |
| Air_Temperature | `0x2CA` (714) | 8:8 | `raw × 0.5 − 40` | °C |
| Steering_Angle | `0x301` (769) | 16:16 | `raw × 0.04395 − 1440.11` | ° |
| Trans_Oil_Temp | `0x39A` (922) | 8:8 | `raw − 48` | °C |
| Oil_Temperature | `0x3F9` (1017) | 40:8 | `raw − 48` | °C |
| Gear | `0x3F9` (1017) | 48:4 | `raw × 1` | — |
| Air_Pressure | `0x3FB` (1019) | 0:8 | `raw × 2 + 598` | mbar |

Wheel speeds are also decoded in mph (`× 0.009703125`) from the same PID `0x254`.

> **Note:** Signals are in Intel (little-endian) byte order unless noted as Motorola. PID mapping data sourced from **RACELOGIC Vehicle CAN Database**, configured for **BMW F22** out of the box. Edit [`can/ptcan.go`](can/ptcan.go) to adjust for your vehicle.

## OLED Display Layout

128×64 pixel monochrome display using proggy TinySZ8pt7b font (6px/char).
Values in four corners with pixel-art icons, G-force visualizer in center.

```
┌────────────────────────────────────────┐
│ 💧95C                         ⚙82C    │
│            ╭──────────────╮            │
│  Y:-5      │      ┼       │     X:8   │
│  m/s2      │   ───●───    │     m/s2  │
│            │      ┼       │            │
│            ╰──────────────╯            │
│ 🌡22C                             3    │
└────────────────────────────────────────┘
```

- **💧** Oil drop (7×8px) — engine oil temperature (top-left)
- **⚙** Gear/cog (8×8px) — transmission oil temperature (top-right)
- **🌡** Thermometer (5×8px) — air temperature (bottom-left)
- **Gear** — plain number (bottom-right)
- **G-force** — crosshair with dot (center), Y/X values in m/s² on each side

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

BMW F-Series PT-CAN runs at **500 kbps**. Connect directly at the DME/DDE or other PT-CAN ECUs.

Connect MCP2515 **CAN-H** and **CAN-L** to the PT-CAN bus. A 120Ω termination resistor may be needed depending on your tap point.

## Project Structure

```
f-tap/
├── main.go               # Entry point, hardware init, main loop
├── can/
│   └── ptcan.go          # BMW PT-CAN message IDs & decoder
├── display/
│   └── oled.go           # OLED rendering (SSD1306/SSD1309)
├── go.mod
├── Makefile
└── README.md
``
