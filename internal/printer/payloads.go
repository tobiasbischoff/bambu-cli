package printer

func PayloadLight(on bool) map[string]any {
	mode := "off"
	if on {
		mode = "on"
	}
	return map[string]any{
		"system": map[string]any{
			"led_mode": mode,
		},
	}
}

func PayloadPrintStop() map[string]any {
	return map[string]any{"print": map[string]any{"command": "stop"}}
}

func PayloadPrintPause() map[string]any {
	return map[string]any{"print": map[string]any{"command": "pause"}}
}

func PayloadPrintResume() map[string]any {
	return map[string]any{"print": map[string]any{"command": "resume"}}
}

func PayloadGcode(line string) map[string]any {
	return map[string]any{
		"print": map[string]any{
			"sequence_id": "0",
			"command":     "gcode_line",
			"param":       line,
		},
	}
}

func PayloadStartPrint(filename string, plateLocation string, useAMS bool, amsMapping []int, skipObjects []int, flowCalibration bool) map[string]any {
	payload := map[string]any{
		"print": map[string]any{
			"command":        "project_file",
			"param":          plateLocation,
			"file":           filename,
			"bed_leveling":   true,
			"bed_type":       "textured_plate",
			"flow_cali":      flowCalibration,
			"vibration_cali": true,
			"url":            "ftp:///" + filename,
			"layer_inspect":  false,
			"sequence_id":    "10000000",
			"use_ams":        useAMS,
			"ams_mapping":    amsMapping,
			"skip_objects":   nil,
		},
	}
	if skipObjects != nil && len(skipObjects) > 0 {
		payload["print"].(map[string]any)["skip_objects"] = skipObjects
	}
	return payload
}

func PayloadCalibration(bedLevel, motorNoise, vibration bool) map[string]any {
	bitmask := 0
	if bedLevel {
		bitmask |= 1 << 1
	}
	if vibration {
		bitmask |= 1 << 2
	}
	if motorNoise {
		bitmask |= 1 << 3
	}
	return map[string]any{"print": map[string]any{"command": "calibration", "option": bitmask}}
}

func PayloadReboot() map[string]any {
	return map[string]any{"system": map[string]any{"command": "reboot"}}
}
