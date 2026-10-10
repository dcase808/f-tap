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
| Battery_Current | `0x1BA` (442) | 24:16 | `raw × 0.02 − 200` | A |
| Wheel_Speed_RL | `0x254` (596) | 0:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_RR | `0x254` (596) | 16:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_FL | `0x254` (596) | 32:15 | `raw × 0.015625` (Signed) | km/h |
| Wheel_Speed_FR | `0x254` (596) | 48:15 | `raw × 0.015625` (Signed) | km/h |
| Battery_Voltage | `0x281` (641) | 0:12 | `raw × 15` | mV |
| Air_Temperature | `0x2CA` (714) | 8:8 | `raw × 0.5 − 40` | °C |
| Steering_Angle | `0x301` (769) | 16:16 | `raw × 0.04395 − 1440.11` | ° |
| Trans_Oil_Temp | `0x39A` (922) | 8:8 | `raw − 48` | °C |
| Oil_Temperature | `0x3F9` (1017) | 40:8 | `raw − 48` | °C |
| Gear | `0x3F9` (1017) | 48:4 | `raw − 4` | — |
| Air_Pressure | `0x3FB` (1019) | 0:8 | `raw × 2 + 598` | mbar |

Wheel speeds are also decoded in mph (`× 0.009703125`) from the same PID `0x254`.

> **Note:** Signals are in Intel (little-endian) byte order unless noted as Motorola. PID mapping data sourced from **RACELOGIC Vehicle CAN Database**, configured for **BMW F22** out of the box. Edit [`can/ptcan.go`](can/ptcan.go) to adjust for your vehicle.

## OLED Display Layout

128×64 pixel monochrome display using proggy TinySZ8pt7b font (6px/char).
Values along the top and bottom edges with pixel-art icons, G-force visualizer in center.

```
┌────────────────────────────────────────┐
│ 💧95C           2500r         ⚙82C    │
│            ╭──────────────╮            │
│  Y:-0.5    │      ┼       │    X:0.8   │
│  m/s2      │   ───●───    │    m/s2    │
│            │      ┼       │            │
│            ╰──────────────╯            │
│ 🌡22C           87kph             3    │
└────────────────────────────────────────┘
```

- **💧** Oil drop (7×8px) — engine oil temperature (top-left)
- **RPM** — engine speed, `r` suffix (top-center)
- **⚙** Gear/cog (8×8px) — transmission oil temperature (top-right)
- **🌡** Thermometer (5×8px) — air temperature (bottom-left)
- **Speed** — vehicle speed in km/h (bottom-center)
- **Gear** — `N`, `1`–`8`, `R` or `P` (bottom-right)
- **G-force** — crosshair with dot (center), Y/X values in m/s² on each side

A value shows `--` until its CAN message has been received. If the bus stays silent for 2 seconds, the display returns to the "Waiting for PT-CAN data" screen. Initialization errors (such as a missing MCP2515) are shown on screen.

## Hardware

| Component           | Description                              |
|---------------------|------------------------------------------|
| Raspberry Pi Pico   | RP2040 microcontroller                   |
| MCP2515 Module      | SPI CAN bus controller (8 MHz crystal)   |
| 2.42" OLED          | SSD1309, 128×64, I2C (0x3C)             |

CAN bit timing is set for an 8 MHz crystal. For a 16 MHz module, change `canCNF1`–`canCNF3` in [`main.go`](main.go).

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

> **Note:** The common MCP2515 breakout carries a TJA1050 transceiver, which is specified for a 4.75–5.25 V supply. If reception is unreliable on 3V3, power the transceiver from 5 V while keeping the SPI lines at 3.3 V (level-shift `SO`; the Pico's GPIOs are not 5 V tolerant), or use a board with a 3.3 V transceiver such as the SN65HVD230.

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

The serial monitor prints bus statistics once per second: `msgs` (frames received), `rx errors` (receive-buffer overflows in the MCP2515, i.e. dropped frames) and `last id` (ID of the last frame, decimal).

`make build TARGET=pico-w` also works, but the Pico W's LED is wired to its wireless chip, so there is no heartbeat or error blink on that board. Errors still appear on the OLED.

## PT-CAN Connection

> ⚠️ **WARNING:** Only connect in **listen-only mode**. Incorrect CAN bus connections can interfere with vehicle systems. Always tap — never inject.

The firmware runs the MCP2515 in Listen-Only mode: it never acknowledges frames, sends error frames or transmits.

BMW F-Series PT-CAN runs at **500 kbps**. Connect directly at the DME/DDE or other PT-CAN ECUs.

Connect MCP2515 **CAN-H** and **CAN-L** to the PT-CAN bus. The car's bus is already terminated at both ends, so do not add termination at the tap: remove the 120Ω jumper on the MCP2515 module. Termination is only needed on a bench setup that has none.

On a bench, keep in mind that a listen-only node does not acknowledge frames. A lone transmitter needs a second, normal node on the bus, or it will keep retransmitting.

## Project Structure

```
f-tap/
├── main.go               # Entry point, hardware init, main loop
├── mcp2515.go            # MCP2515 Listen-Only setup & frame reception
├── can/
│   └── ptcan.go          # BMW PT-CAN message IDs & decoder
├── display/
│   └── oled.go           # OLED rendering (SSD1306/SSD1309)
├── go.mod
├── Makefile
└── README.md
```
