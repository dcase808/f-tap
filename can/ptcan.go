// Package can provides BMW F-Series PT-CAN message decoding.
//
// PT-CAN (Powertrain CAN) runs at 500 kbps and carries engine/drivetrain data.
// CAN IDs sourced from opendbc (commaai/opendbc bmw_e9x_e8x.dbc) which is
// confirmed to also cover many F-Series models. Verify on your specific car.
package can

// PT-CAN Message IDs (BMW F/E-Series, from opendbc)
const (
	// IDEngineData (0x1D0 / 464) from DME — carries temperatures and pressures.
	//   Byte 0: TEMP_ENG — Engine coolant temperature (raw - 48 = °C)
	//   Byte 1: TEMP_EOI — Engine oil temperature (raw - 48 = °C)
	//   Byte 3: AIP_ENG  — Intake air pressure (raw × 2 + 598 = hPa)
	//   Bytes 2-3: Counter, warm-up status, engine run state
	//   Bytes 4-5: IJV_FU — Fuel injector value (raw - 48, °C units)
	//   Byte 7: RPM_IDLG_TAR — Target idle RPM (raw × 5)
	IDEngineData uint32 = 0x1D0

	// IDGearboxTorque (0x0B5 / 181) from EGS — torque request + gearbox temp.
	//   Byte 7: Gearbox_temperature (raw = °C)
	IDGearboxTorque uint32 = 0x0B5
)

// Atmospheric pressure reference for boost calculation (mbar).
const AtmosphericPressureMbar = 1013

// VehicleData holds the most recent decoded values from PT-CAN.
type VehicleData struct {
	CoolantTemp    int16  // Engine coolant temperature in °C
	OilTemp        int16  // Engine oil temperature in °C
	GearboxOilTemp int16  // Gearbox (transmission) oil temperature in °C
	IntakeAirPress int16  // Intake manifold absolute pressure in hPa
	BoostMbar      int16  // Boost pressure in mbar (MAP - atmospheric; negative = vacuum)
	MsgCount       uint32 // Total CAN messages received
	LastID         uint32 // Last CAN ID seen
	Initialized    bool   // True once any valid data has been parsed
}

// ParseMessage decodes a raw CAN message and updates VehicleData.
// Returns true if the message ID was recognized and parsed.
func (v *VehicleData) ParseMessage(id uint32, data []byte) bool {
	v.MsgCount++
	v.LastID = id

	switch id {
	case IDEngineData:
		if len(data) >= 4 {
			// Byte 0: Coolant temp — raw - 48 = °C
			v.CoolantTemp = int16(data[0]) - 48

			// Byte 1: Oil temp — raw - 48 = °C
			v.OilTemp = int16(data[1]) - 48

			// Byte 3: Intake air pressure — raw × 2 + 598 = hPa
			v.IntakeAirPress = int16(data[3])*2 + 598

			// Boost = MAP - atmospheric (in mbar, 1 hPa = 1 mbar)
			v.BoostMbar = v.IntakeAirPress - AtmosphericPressureMbar

			v.Initialized = true
		}
		return true

	case IDGearboxTorque:
		if len(data) >= 8 {
			// Byte 7: Gearbox oil temperature in °C
			v.GearboxOilTemp = int16(data[7])
			v.Initialized = true
		}
		return true
	}

	return false
}
