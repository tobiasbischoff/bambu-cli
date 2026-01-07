package printer

import "fmt"

type PrintStatus int

const (
	PrintStatusPrinting                     PrintStatus = 0
	PrintStatusAutoBedLeveling              PrintStatus = 1
	PrintStatusHeatbedPreheating            PrintStatus = 2
	PrintStatusSweepingXYMechMode           PrintStatus = 3
	PrintStatusChangingFilament             PrintStatus = 4
	PrintStatusM400Pause                    PrintStatus = 5
	PrintStatusPausedFilamentRunout         PrintStatus = 6
	PrintStatusHeatingHotend                PrintStatus = 7
	PrintStatusCalibratingExtrusion         PrintStatus = 8
	PrintStatusScanningBedSurface           PrintStatus = 9
	PrintStatusInspectingFirstLayer         PrintStatus = 10
	PrintStatusIdentifyingBuildPlateType    PrintStatus = 11
	PrintStatusCalibratingMicroLidar        PrintStatus = 12
	PrintStatusHomingToolhead               PrintStatus = 13
	PrintStatusCleaningNozzleTip            PrintStatus = 14
	PrintStatusCheckingExtruderTemperature  PrintStatus = 15
	PrintStatusPausedUser                   PrintStatus = 16
	PrintStatusPausedFrontCoverFalling      PrintStatus = 17
	PrintStatusCalibratingLidar             PrintStatus = 18
	PrintStatusCalibratingExtrusionFlow     PrintStatus = 19
	PrintStatusPausedNozzleTempMalfunction  PrintStatus = 20
	PrintStatusPausedHeatBedTempMalfunction PrintStatus = 21
	PrintStatusFilamentUnloading            PrintStatus = 22
	PrintStatusPausedSkippedStep            PrintStatus = 23
	PrintStatusFilamentLoading              PrintStatus = 24
	PrintStatusCalibratingMotorNoise        PrintStatus = 25
	PrintStatusPausedAMSLost                PrintStatus = 26
	PrintStatusPausedLowFanSpeedHeatBreak   PrintStatus = 27
	PrintStatusPausedChamberTempControl     PrintStatus = 28
	PrintStatusCoolingChamber               PrintStatus = 29
	PrintStatusPausedUserGcode              PrintStatus = 30
	PrintStatusMotorNoiseShowoff            PrintStatus = 31
	PrintStatusPausedNozzleFilamentCovered  PrintStatus = 32
	PrintStatusPausedCutterError            PrintStatus = 33
	PrintStatusPausedFirstLayerError        PrintStatus = 34
	PrintStatusPausedNozzleClog             PrintStatus = 35
	PrintStatusIdle                         PrintStatus = 255
)

func (s PrintStatus) String() string {
	if s == PrintStatusIdle {
		return "IDLE"
	}
	name, ok := printStatusNames[s]
	if !ok {
		return "UNKNOWN"
	}
	return name
}

var printStatusNames = map[PrintStatus]string{
	PrintStatusPrinting:                     "PRINTING",
	PrintStatusAutoBedLeveling:              "AUTO_BED_LEVELING",
	PrintStatusHeatbedPreheating:            "HEATBED_PREHEATING",
	PrintStatusSweepingXYMechMode:           "SWEEPING_XY_MECH_MODE",
	PrintStatusChangingFilament:             "CHANGING_FILAMENT",
	PrintStatusM400Pause:                    "M400_PAUSE",
	PrintStatusPausedFilamentRunout:         "PAUSED_FILAMENT_RUNOUT",
	PrintStatusHeatingHotend:                "HEATING_HOTEND",
	PrintStatusCalibratingExtrusion:         "CALIBRATING_EXTRUSION",
	PrintStatusScanningBedSurface:           "SCANNING_BED_SURFACE",
	PrintStatusInspectingFirstLayer:         "INSPECTING_FIRST_LAYER",
	PrintStatusIdentifyingBuildPlateType:    "IDENTIFYING_BUILD_PLATE_TYPE",
	PrintStatusCalibratingMicroLidar:        "CALIBRATING_MICRO_LIDAR",
	PrintStatusHomingToolhead:               "HOMING_TOOLHEAD",
	PrintStatusCleaningNozzleTip:            "CLEANING_NOZZLE_TIP",
	PrintStatusCheckingExtruderTemperature:  "CHECKING_EXTRUDER_TEMPERATURE",
	PrintStatusPausedUser:                   "PAUSED_USER",
	PrintStatusPausedFrontCoverFalling:      "PAUSED_FRONT_COVER_FALLING",
	PrintStatusCalibratingLidar:             "CALIBRATING_LIDAR",
	PrintStatusCalibratingExtrusionFlow:     "CALIBRATING_EXTRUSION_FLOW",
	PrintStatusPausedNozzleTempMalfunction:  "PAUSED_NOZZLE_TEMPERATURE_MALFUNCTION",
	PrintStatusPausedHeatBedTempMalfunction: "PAUSED_HEAT_BED_TEMPERATURE_MALFUNCTION",
	PrintStatusFilamentUnloading:            "FILAMENT_UNLOADING",
	PrintStatusPausedSkippedStep:            "PAUSED_SKIPPED_STEP",
	PrintStatusFilamentLoading:              "FILAMENT_LOADING",
	PrintStatusCalibratingMotorNoise:        "CALIBRATING_MOTOR_NOISE",
	PrintStatusPausedAMSLost:                "PAUSED_AMS_LOST",
	PrintStatusPausedLowFanSpeedHeatBreak:   "PAUSED_LOW_FAN_SPEED_HEAT_BREAK",
	PrintStatusPausedChamberTempControl:     "PAUSED_CHAMBER_TEMPERATURE_CONTROL_ERROR",
	PrintStatusCoolingChamber:               "COOLING_CHAMBER",
	PrintStatusPausedUserGcode:              "PAUSED_USER_GCODE",
	PrintStatusMotorNoiseShowoff:            "MOTOR_NOISE_SHOWOFF",
	PrintStatusPausedNozzleFilamentCovered:  "PAUSED_NOZZLE_FILAMENT_COVERED_DETECTED",
	PrintStatusPausedCutterError:            "PAUSED_CUTTER_ERROR",
	PrintStatusPausedFirstLayerError:        "PAUSED_FIRST_LAYER_ERROR",
	PrintStatusPausedNozzleClog:             "PAUSED_NOZZLE_CLOG",
}

type GcodeState string

const (
	GcodeStateIdle    GcodeState = "IDLE"
	GcodeStatePrepare GcodeState = "PREPARE"
	GcodeStateRunning GcodeState = "RUNNING"
	GcodeStatePause   GcodeState = "PAUSE"
	GcodeStateFinish  GcodeState = "FINISH"
	GcodeStateFailed  GcodeState = "FAILED"
	GcodeStateUnknown GcodeState = "UNKNOWN"
)

func ParseGcodeState(v any) GcodeState {
	s, ok := v.(string)
	if !ok {
		return GcodeStateUnknown
	}
	switch s {
	case string(GcodeStateIdle):
		return GcodeStateIdle
	case string(GcodeStatePrepare):
		return GcodeStatePrepare
	case string(GcodeStateRunning):
		return GcodeStateRunning
	case string(GcodeStatePause):
		return GcodeStatePause
	case string(GcodeStateFinish):
		return GcodeStateFinish
	case string(GcodeStateFailed):
		return GcodeStateFailed
	default:
		return GcodeStateUnknown
	}
}

func ParsePrintStatus(v any) (PrintStatus, error) {
	switch val := v.(type) {
	case float64:
		return PrintStatus(int(val)), nil
	case int:
		return PrintStatus(val), nil
	case int64:
		return PrintStatus(int(val)), nil
	case jsonNumber:
		i, err := val.Int64()
		if err != nil {
			return PrintStatus(0), err
		}
		return PrintStatus(int(i)), nil
	default:
		return PrintStatus(0), fmt.Errorf("unsupported status type")
	}
}

// jsonNumber is satisfied by encoding/json.Number without importing it here.
type jsonNumber interface {
	Int64() (int64, error)
}
