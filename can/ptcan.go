// Package can provides BMW F-Series PT-CAN message decoding.
//
// PT-CAN (Powertrain CAN) runs at 500 kbps and carries engine/drivetrain data.
// Signal definitions (PID, bit position, scale, offset, signedness) are derived
// from DBC analysis. Verify on your specific car.
package can

import "sync"

// PT-CAN Message IDs (decimal → hex)
const (
	IDThrottlePosition uint32 = 0x0D9 // 217
	IDEngineSpeed      uint32 = 0x0F3 // 243
	IDBrake            uint32 = 0x173 // 371 — Brake_Position + Brake_Light
	IDLongAccel        uint32 = 0x199 // 409
	IDLatAccel         uint32 = 0x19A // 410
	IDVehicleSpeed     uint32 = 0x1A1 // 417 — kph & mph
	IDBatteryCurrent   uint32 = 0x1BA // 442
	IDWheelSpeeds      uint32 = 0x254 // 596 — FL/FR/RL/RR (kph & mph)
	IDBatteryVoltage   uint32 = 0x281 // 641
	IDAirTemperature   uint32 = 0x2CA // 714
	IDSteeringAngle    uint32 = 0x301 // 769
	IDTransOilTemp     uint32 = 0x39A // 922
	IDGearOilTemp      uint32 = 0x3F9 // 1017 — Gear + Oil_Temperature
	IDAirPressure      uint32 = 0x3FB // 1019
)

// Signal decoding constants (from DBC analysis)
const (
	airPressureOffset   float32 = 598.0
	steeringAngleScale  float32 = 0.04395
	steeringAngleOffset float32 = 1440.11
	speedScaleKph       float32 = 0.015625    // 1/64
	speedScaleMph       float32 = 0.009703125 // kph scale × 0.621371
)

// VehicleState holds the actual data without the mutex.
type VehicleState struct {
	// Temperatures
	AirTemp      float32 // °C  (PID 714)
	OilTemp      float32 // °C  (PID 1017)
	TransOilTemp float32 // °C  (PID 922)

	// Pressures
	AirPressure float32 // mbar (PID 1019)

	// Battery
	BatteryVoltage float32 // mV  (PID 641)
	BatteryCurrent float32 // A   (PID 442)

	// Brakes
	BrakePosition float32 //         (PID 371)
	BrakeLight    float32 // on/off  (PID 371)

	// Drivetrain
	Gear        float32 //     (PID 1017)
	EngineSpeed float32 // rpm (PID 243)
	ThrottlePos float32 // %   (PID 217)

	// Dynamics
	LongAccel float32 // m/s² (PID 409)
	LatAccel  float32 // m/s² (PID 410)

	// Vehicle speed
	VehicleSpeedKph float32 // km/h (PID 417)
	VehicleSpeedMph float32 // mph  (PID 417)

	// Steering
	SteeringAngle float32 // °   (PID 769)

	// Wheel speeds
	WheelSpeedFLKph float32 // km/h (PID 596)
	WheelSpeedFRKph float32 // km/h (PID 596)
	WheelSpeedRLKph float32 // km/h (PID 596)
	WheelSpeedRRKph float32 // km/h (PID 596)
	WheelSpeedFLMph float32 // mph  (PID 596)
	WheelSpeedFRMph float32 // mph  (PID 596)
	WheelSpeedRLMph float32 // mph  (PID 596)
	WheelSpeedRRMph float32 // mph  (PID 596)

	// Metadata
	MsgCount    uint32 // Total CAN messages received
	RxErrors    uint32 // Total CAN receive errors
	LastID      uint32 // Last CAN ID seen
	Initialized bool   // True once any valid data has been parsed
}

// VehicleData holds the most recent decoded values from PT-CAN.
// Float fields use float32 to preserve fractional scales on an RP2040.
type VehicleData struct {
	sync.RWMutex
	VehicleState
}

// ParseMessage decodes a raw CAN message and updates VehicleData.
// Returns true if the message ID was recognized and parsed.
func (v *VehicleData) ParseMessage(id uint32, data []byte) bool {
	v.Lock()
	defer v.Unlock()

	v.MsgCount++
	v.LastID = id

	switch id {

	// ── Air_Pressure (PID 1019) ──
	// StartBit=0, BitLen=8, Offset=598, Scale=2, Unsigned, Intel
	case IDAirPressure:
		if len(data) >= 1 {
			raw := uint16(data[0])
			v.AirPressure = float32(raw)*2 + airPressureOffset
			v.Initialized = true
		}
		return true

	// ── Air_Temperature (PID 714) ──
	// StartBit=8, BitLen=8, Offset=-40, Scale=0.5, Unsigned, Intel
	case IDAirTemperature:
		if len(data) >= 2 {
			raw := uint16(data[1])
			v.AirTemp = float32(raw)*0.5 - 40
			v.Initialized = true
		}
		return true

	// ── Battery_Voltage (PID 641) ──
	// StartBit=0, BitLen=12, Offset=0, Scale=15, Unsigned, Intel
	case IDBatteryVoltage:
		if len(data) >= 2 {
			raw := uint16(data[0]) | (uint16(data[1]) << 8)
			raw &= 0x0FFF // 12 bits
			v.BatteryVoltage = float32(raw) * 15
			v.Initialized = true
		}
		return true

	// ── Brake_Position + Brake_Light (PID 371) ──
	case IDBrake:
		if len(data) >= 8 {
			// Brake_Position: StartBit=56, BitLen=6, Offset=0, Scale=1, Unsigned, Intel
			raw := uint16(data[7])
			raw &= 0x3F // 6 bits
			v.BrakePosition = float32(raw)

			// Brake_Light: StartBit=3, BitLen=2, Offset=0, Scale=0.5, Unsigned, Motorola
			// Motorola bit 3 in a single byte → byte 0, bits [3:2]
			rawBL := uint16((data[0] >> 2) & 0x03)
			v.BrakeLight = float32(rawBL) * 0.5

			v.Initialized = true
		}
		return true

	// ── Battery_Current (PID 442) ──
	// StartBit=24, BitLen=16, Offset=-200, Scale=0.02, Unsigned, Intel
	case IDBatteryCurrent:
		if len(data) >= 5 {
			raw := uint16(data[3]) | (uint16(data[4]) << 8)
			v.BatteryCurrent = float32(raw)*0.02 - 200
			v.Initialized = true
		}
		return true

	// ── Gear + Oil_Temperature (PID 1017) ──
	case IDGearOilTemp:
		if len(data) >= 7 {
			// Gear: StartBit=48, BitLen=4, Offset=0, Scale=1, Unsigned, Intel
			rawGear := uint16(data[6]) & 0x0F
			v.Gear = float32(rawGear)

			// Oil_Temperature: StartBit=40, BitLen=8, Offset=-48, Scale=1, Unsigned, Intel
			rawOil := uint16(data[5])
			v.OilTemp = float32(rawOil) - 48

			v.Initialized = true
		}
		return true

	// ── Transmission_Oil_Temperature (PID 922) ──
	// StartBit=8, BitLen=8, Offset=-48, Scale=1, Unsigned, Intel
	case IDTransOilTemp:
		if len(data) >= 2 {
			raw := uint16(data[1])
			v.TransOilTemp = float32(raw) - 48
			v.Initialized = true
		}
		return true

	// ── Longitudinal Acceleration (PID 409) ──
	// StartBit=16, BitLen=16, Offset=-65, Scale=0.002, Unsigned, Intel
	case IDLongAccel:
		if len(data) >= 4 {
			raw := uint16(data[2]) | (uint16(data[3]) << 8)
			v.LongAccel = float32(raw)*0.002 - 65
			v.Initialized = true
		}
		return true

	// ── Lateral Acceleration (PID 410) ──
	// StartBit=16, BitLen=16, Offset=-65, Scale=0.002, Unsigned, Intel
	case IDLatAccel:
		if len(data) >= 4 {
			raw := uint16(data[2]) | (uint16(data[3]) << 8)
			v.LatAccel = float32(raw)*0.002 - 65
			v.Initialized = true
		}
		return true

	// ── Engine_Speed (PID 243) ──
	// StartBit=12, BitLen=12, Offset=0, Scale=10, Unsigned, Intel
	case IDEngineSpeed:
		if len(data) >= 3 {
			// Bits 12..23 across bytes 1-2 (Intel byte order)
			raw := (uint16(data[1]) >> 4) | (uint16(data[2]) << 4)
			raw &= 0x0FFF // 12 bits
			v.EngineSpeed = float32(raw) * 10
			v.Initialized = true
		}
		return true

	// ── Vehicle Speed (PID 417) — kph + mph ──
	case IDVehicleSpeed:
		if len(data) >= 4 {
			raw := uint16(data[2]) | (uint16(data[3]) << 8)

			// Indicated_Vehicle_Speed_kph: Scale=0.015625
			v.VehicleSpeedKph = float32(raw) * speedScaleKph

			// Indicated_Vehicle_Speed_mph: Scale=0.009703125
			v.VehicleSpeedMph = float32(raw) * speedScaleMph

			v.Initialized = true
		}
		return true

	// ── Wheel Speeds (PID 596) — FL/FR/RL/RR, kph + mph ──
	// All 15-bit Signed Intel; RL=bit0, RR=bit16, FL=bit32, FR=bit48
	case IDWheelSpeeds:
		if len(data) >= 8 {
			rawRL := signExtend15(uint16(data[0]) | (uint16(data[1]) << 8))
			rawRR := signExtend15(uint16(data[2]) | (uint16(data[3]) << 8))
			rawFL := signExtend15(uint16(data[4]) | (uint16(data[5]) << 8))
			rawFR := signExtend15(uint16(data[6]) | (uint16(data[7]) << 8))

			v.WheelSpeedRLKph = float32(rawRL) * speedScaleKph
			v.WheelSpeedRRKph = float32(rawRR) * speedScaleKph
			v.WheelSpeedFLKph = float32(rawFL) * speedScaleKph
			v.WheelSpeedFRKph = float32(rawFR) * speedScaleKph

			v.WheelSpeedRLMph = float32(rawRL) * speedScaleMph
			v.WheelSpeedRRMph = float32(rawRR) * speedScaleMph
			v.WheelSpeedFLMph = float32(rawFL) * speedScaleMph
			v.WheelSpeedFRMph = float32(rawFR) * speedScaleMph

			v.Initialized = true
		}
		return true

	// ── Steering_Angle (PID 769) ──
	// StartBit=16, BitLen=16, Offset=-1440.11, Scale=0.04395, Unsigned, Intel
	case IDSteeringAngle:
		if len(data) >= 4 {
			raw := uint16(data[2]) | (uint16(data[3]) << 8)
			v.SteeringAngle = float32(raw)*steeringAngleScale - steeringAngleOffset
			v.Initialized = true
		}
		return true

	// ── Throttle_Position (PID 217) ──
	// StartBit=16, BitLen=12, Offset=0, Scale=0.025, Unsigned, Intel
	case IDThrottlePosition:
		if len(data) >= 4 {
			raw := uint16(data[2]) | (uint16(data[3]) << 8)
			raw &= 0x0FFF // 12 bits
			v.ThrottlePos = float32(raw) * 0.025
			v.Initialized = true
		}
		return true
	}

	return false
}

// signExtend15 sign-extends a 15-bit unsigned value to int16.
func signExtend15(raw uint16) int16 {
	v := int16(raw & 0x7FFF)
	if v&0x4000 != 0 {
		v |= ^int16(0x3FFF)
	}
	return v
}

// IncrementRxErrors atomically increments the CAN receive error counter.
func (v *VehicleData) IncrementRxErrors() {
	v.Lock()
	defer v.Unlock()
	v.RxErrors++
}

// Snapshot returns a point-in-time copy of the vehicle data.
// The caller receives a plain value with no lock references.
func (v *VehicleData) Snapshot() VehicleState {
	v.RLock()
	defer v.RUnlock()

	return v.VehicleState
}
