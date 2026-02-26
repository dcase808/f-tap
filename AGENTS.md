# AGENTS.md — F-TAP Coding Agent Reference

## Project Overview

**F-TAP** is a TinyGo firmware project for the Raspberry Pi Pico (RP2040). It taps into a BMW
F-Series PT-CAN bus at 500 kbps via an MCP2515 SPI CAN controller, decodes ~20 vehicle signals,
and renders them on a 2.42" SSD1309 128×64 OLED display over I2C.

- **Language:** Go, compiled with [TinyGo](https://tinygo.org/) — NOT standard `go build`
- **Target:** Raspberry Pi Pico (RP2040), bare-metal embedded (no OS)
- **Go version:** 1.22.0
- **Module:** `github.com/dcase808/f-tap`
- **Packages:** `main`, `can`, `display`

---

## Build, Flash & Monitor Commands

```bash
make build            # Compile → build/f-tap.uf2  (requires TinyGo)
make flash            # Compile & flash to Pico (hold BOOTSEL, then plug USB)
make monitor          # Open serial monitor at 115200 baud
make clean            # Remove build/ directory

# Override target board:
make build TARGET=pico-w

# Equivalent manual commands:
tinygo build -target=pico -size short -o build/f-tap.uf2 .
tinygo flash -target=pico -size short .
tinygo monitor -baudrate=115200
```

---

## Testing

**There is no automated test suite.** TinyGo on bare-metal embedded targets does not support the
standard `go test` runner, and the `testing` package is unavailable. No `*_test.go` files exist in
this repository.

Testing is done by flashing firmware to hardware and observing serial output or display behavior:

```bash
make flash && make monitor
```

If you need to validate logic changes, ensure the firmware builds without error (`make build`) and
reason through the logic manually. Do not add `*_test.go` files — they will not compile under
TinyGo for the `pico` target.

---

## Linting & Formatting

No linter is configured. Apply standard Go formatting rules:

```bash
gofmt -w .           # Format all Go files in place
goimports -w .       # Format + fix imports (preferred if available)
```

There is no `.golangci.yml`, `staticcheck.conf`, or `.prettierrc`. Follow `gofmt` output exactly —
do not diverge from it.

---

## Code Style Guidelines

### Imports

Group imports in three blocks, separated by blank lines, in this order:

1. Standard library (including `machine` and `time` from TinyGo's stdlib-compatible layer)
2. External packages (`tinygo.org/x/...`)
3. Internal packages (`github.com/dcase808/f-tap/...`)

```go
import (
    "machine"
    "time"

    "tinygo.org/x/drivers/mcp2515"

    "github.com/dcase808/f-tap/can"
    "github.com/dcase808/f-tap/display"
)
```

Never use dot imports or blank imports unless justified by a TinyGo driver requirement.

### Naming Conventions

| Symbol | Convention | Example |
|---|---|---|
| Exported types/structs | `PascalCase` | `VehicleData`, `OLED` |
| Exported functions/methods | `PascalCase` | `ParseMessage`, `Snapshot`, `Render` |
| Unexported functions/methods | `camelCase` | `drawIcon`, `blinkError`, `fmtF32` |
| Exported constants | `PascalCase` | `IDEngineSpeed`, `IDThrottlePosition` |
| Unexported constants/vars | `camelCase` | `gCenterX`, `smoothingAlpha`, `screenWidth` |
| Local variables | `camelCase` | `vehicleData`, `gearStr`, `countStr` |
| Method receivers | Single lowercase letter matching type initial | `v *VehicleData`, `o *OLED` |
| Constructor functions | `New(...)` | `display.New(...)` |
| CAN ID constants | `ID` prefix | `IDEngineSpeed`, `IDBrake` |
| Icon bitmaps | `icon` prefix | `iconOil`, `iconGear`, `iconThermo` |
| Packages | Lowercase, single word | `can`, `display` |
| Files | `snake_case.go` | `ptcan.go`, `oled.go` |

### Types

- Always use `float32`, never `float64` — the RP2040 has a hardware float32 unit.
- Use explicit integer widths for all hardware values: `uint8`, `uint16`, `int16`, `uint32`,
  `int32`, `int64`, `uint64`. Never use bare `int` for CAN signal values or hardware registers.
- Icon bitmaps are `[8]byte` (fixed-size arrays); pass as `[]byte` slices to drawing functions.
- Avoid `interface{}` / `any` — there is no reflection support in TinyGo embedded targets.

### No `fmt` Package

The `fmt` package is **not available** in TinyGo bare-metal builds. Use `strconv` instead:

```go
// Correct
s := strconv.FormatInt(int64(val), 10)
s := strconv.FormatUint(uint64(val), 10)

// Wrong — will not compile
s := fmt.Sprintf("%d", val)
```

Similarly, `log`, `os`, and most `net` packages are unavailable.

### Comments

- Every exported symbol must have a Go doc comment (starts with the symbol name).
- Use section separator comments to divide logical regions inside long functions or files:
  ```go
  // ── Section Name ──────────────────────────────────────────────
  ```
- Annotate `VehicleData` fields with unit and PID source:
  ```go
  EngineTemp float32 // °C  (PID 0x4E2)
  ```
- Annotate each `case` in CAN signal switch statements with signal metadata:
  ```go
  // EngineSpeed — ID 0x0AA | StartBit 0 | BitLen 16 | Scale 0.25 | Unit rpm
  ```

### Error Handling

This is a bare-metal embedded system — there is no `panic` recovery, no `log.Fatal`, no `os.Exit`.

**Hardware initialization errors** block forever with a visual LED indicator:

```go
err := canDev.Configure(mcp2515.Config{...})
if err != nil {
    blinkError(led)   // infinite loop — device must be physically reset
}
```

**Runtime / CAN receive errors** are silently skipped to avoid halting the read loop:

```go
msg, err := canDev.Rx()
if err == nil && msg != nil {
    data.ParseMessage(msg.ID, msg.Data)
}
```

**Byte-slice bounds** must always be checked before decoding:

```go
if len(data) >= 4 {
    // safe to read data[0..3]
}
```

Never use `panic`, never index a slice without a prior length check.

### Concurrency

- `sync.RWMutex` is embedded directly inside structs that are shared across goroutines.
- Writers call `v.Lock()` / `defer v.Unlock()`; readers call `v.RLock()` / `defer v.RUnlock()`.
- Use the **Snapshot pattern**: `Snapshot()` returns a full value copy so the display goroutine
  holds no lock during slow I2C transfers.
- Two goroutines are launched from `main`: `go readCAN(...)` and `go renderDisplay(...)`. The main
  goroutine then becomes the LED heartbeat loop — do not block it permanently elsewhere.

---

## Architecture Notes

```
main.go          Hardware init (I2C, SPI, MCP2515 @ 500 kbps), goroutine launch, LED heartbeat
can/ptcan.go     PT-CAN message IDs, VehicleData struct, ParseMessage(), Snapshot()
display/oled.go  OLED struct, Render(), ShowSplash(), ShowWaiting(), drawing helpers
```

- `can.VehicleData` is the single shared state object — always access it via its mutex methods.
- `display.OLED` wraps `ssd1306.Device` and owns all drawing logic.
- Do not add dependencies that pull in `net`, `os`, `syscall`, or large stdlib packages — they are
  not available in TinyGo bare-metal builds. Check TinyGo driver compatibility before adding any
  new `go.mod` dependency.

---

## Commit Message Style

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: <short description>
fix: <short description>
refactor: <short description>
docs: <short description>
chore: <short description>
```

Examples from this repo:
- `feat: decrease display refresh interval to 50ms`
- `refactor: extract signal decoding constants and add G-force EMA smoothing`
- `docs: specify BMW F22 configuration`
