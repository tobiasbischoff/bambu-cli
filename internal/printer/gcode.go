package printer

import "regexp"

var (
	gcodeHead  = regexp.MustCompile(`^[GM]\d+`)
	gcodeParam = regexp.MustCompile(`^[A-Z]-?\d+(\.\d+)?$`)
)

func ValidateGcodeLine(line string) bool {
	line = stripComment(line)
	if line == "" {
		return false
	}
	if !gcodeHead.MatchString(line) {
		return false
	}
	parts := splitFields(line)
	for _, p := range parts[1:] {
		if !gcodeParam.MatchString(p) {
			return false
		}
	}
	return true
}

func stripComment(line string) string {
	for i, r := range line {
		if r == ';' {
			return trimSpace(line[:i])
		}
	}
	return trimSpace(line)
}

func splitFields(line string) []string {
	var fields []string
	start := -1
	for i, r := range line {
		if r == ' ' || r == '\t' {
			if start >= 0 {
				fields = append(fields, line[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, line[start:])
	}
	return fields
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) {
		if s[start] != ' ' && s[start] != '\t' && s[start] != '\n' && s[start] != '\r' {
			break
		}
		start++
	}
	end := len(s)
	for end > start {
		c := s[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return s[start:end]
}
