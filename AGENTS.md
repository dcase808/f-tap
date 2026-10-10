# AGENTS.md — F-TAP

## Toolchain
- TinyGo firmware for Raspberry Pi Pico (RP2040). Use TinyGo/Make; standard `go build` and `go test` are not this repo's normal workflow.
- Build output is `build/f-tap.uf2`; `build/` is generated.

## Commands
- `make build` → `tinygo build -target=$(TARGET) -size short -o build/f-tap.uf2 .`
- `make flash` → `tinygo flash -target=$(TARGET) -size short .`
- `make monitor` → `tinygo monitor -baudrate=115200`
- `make clean` → remove `build/`
- Override board with `TARGET=pico-w` (default `pico`).

## Verification
- No automated tests, CI workflows, pre-commit hooks, or lint/typecheck config are checked in; no `*_test.go` files exist.
- If asked for local verification, use `make build`. Real validation is on hardware: `make flash && make monitor`.

## Code map
- `main.go`: only entrypoint; configures I2C OLED + SPI MCP2515, shows splash, starts `readCAN` and `renderDisplay`, then stays in the LED heartbeat loop (which also prints bus statistics on serial).
- `mcp2515.go`: direct SPI access to the MCP2515 (Listen-Only bring-up, frame reception, RX overflow check), used instead of the TinyGo `mcp2515` driver; after init only `readCAN` may touch it.
- `can/ptcan.go`: PT-CAN IDs, decode logic, shared `VehicleData`, and vehicle-specific signal mapping.
- `display/oled.go`: OLED rendering, layout, icons, and formatting helpers.

## Non-obvious constraints
- Preserve the snapshot concurrency pattern: `VehicleData` embeds `sync.RWMutex`; display code renders a `Snapshot()` copy so slow I2C writes do not hold locks.
- Keep CAN decode guards: every indexed payload read is preceded by a length check.
- Match existing numeric choices: `float32` for decoded values, explicit integer widths for CAN/hardware data.
- README describes this as a passive PT-CAN sniffer for BMW F-Series (BMW F22 mapping by default); avoid changes that would make the device transmit on CAN unless explicitly requested. That includes keeping the MCP2515 in Listen-Only mode: the TinyGo `mcp2515` driver's `Begin` enters Normal mode, which ACKs frames and sends error frames.
- A decoded `VehicleState` field is only meaningful while its message's `Has…` flag is set in `Valid`; check `Has` before displaying a value (flags are cleared when the bus goes silent).
- Match local import grouping: stdlib, `tinygo.org/x/...`, then `github.com/dcase808/f-tap/...`.
- Recent history uses Conventional Commits (`fix:`, `refactor:`, `docs:`).
