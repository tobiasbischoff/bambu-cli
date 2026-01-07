package printer

import (
	"fmt"
)

type Status struct {
	GcodeState       GcodeState `json:"gcode_state"`
	PrintStatus      string     `json:"print_status"`
	Percent          int        `json:"percent"`
	LayerCurrent     int        `json:"layer_current"`
	LayerTotal       int        `json:"layer_total"`
	BedTemp          float64    `json:"bed_temp"`
	NozzleTemp       float64    `json:"nozzle_temp"`
	ChamberTemp      float64    `json:"chamber_temp"`
	RemainingMinutes *int       `json:"remaining_minutes,omitempty"`
	File             string     `json:"file"`
	Light            string     `json:"light"`
	WifiSignal       string     `json:"wifi_signal"`
	ErrorCode        int        `json:"error_code"`
}

func GetStatus(c *MQTTClient) Status {
	status := Status{}
	if v, ok := c.Get("print", "gcode_state"); ok {
		status.GcodeState = ParseGcodeState(v)
	}
	if v, ok := c.Get("print", "stg_cur"); ok {
		if ps, err := ParsePrintStatus(v); err == nil {
			status.PrintStatus = ps.String()
		} else {
			status.PrintStatus = "UNKNOWN"
		}
	}
	status.Percent = intValue(c.Get("print", "mc_percent"))
	status.LayerCurrent = intValue(c.Get("print", "layer_num"))
	status.LayerTotal = intValue(c.Get("print", "total_layer_num"))
	status.BedTemp = floatValue(c.Get("print", "bed_temper"))
	status.NozzleTemp = floatValue(c.Get("print", "nozzle_temper"))
	status.ChamberTemp = chamberTemp(c)
	status.File = stringValue(c.Get("print", "gcode_file"))
	status.Light = lightState(c)
	status.WifiSignal = stringValue(c.Get("print", "wifi_signal"))
	status.ErrorCode = intValue(c.Get("print", "print_error"))

	if v, ok := c.Get("print", "mc_remaining_time"); ok {
		if i, okInt := asInt(v); okInt {
			status.RemainingMinutes = &i
		}
	}
	return status
}

func lightState(c *MQTTClient) string {
	v, ok := c.Get("print", "lights_report")
	if !ok {
		return "unknown"
	}
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return "unknown"
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return "unknown"
	}
	mode, ok := first["mode"].(string)
	if !ok {
		return "unknown"
	}
	return mode
}

func chamberTemp(c *MQTTClient) float64 {
	if v, ok := c.Get("print", "chamber_temper"); ok {
		if f, okf := asFloat(v); okf {
			return f
		}
	}
	// fallback to print.device.ctc.info.temp
	if v, ok := c.Get("print", "device"); ok {
		if m, ok := v.(map[string]any); ok {
			if ctc, ok := m["ctc"].(map[string]any); ok {
				if info, ok := ctc["info"].(map[string]any); ok {
					if f, okf := asFloat(info["temp"]); okf {
						return f
					}
				}
			}
		}
	}
	return 0
}

func intValue(v any, ok bool) int {
	if !ok {
		return 0
	}
	if i, ok := asInt(v); ok {
		return i
	}
	return 0
}

func floatValue(v any, ok bool) float64 {
	if !ok {
		return 0
	}
	if f, ok := asFloat(v); ok {
		return f
	}
	return 0
}

func stringValue(v any, ok bool) string {
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case jsonNumber:
		i, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case jsonNumber:
		f, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return float64(f), true
	default:
		return 0, false
	}
}
