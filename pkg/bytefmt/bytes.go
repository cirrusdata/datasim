package bytefmt

import (
	"fmt"
	"strconv"
	"strings"
)

var multipliers = map[string]int64{
	"b":   1,
	"k":   1000,
	"kb":  1000,
	"m":   1000 * 1000,
	"mb":  1000 * 1000,
	"g":   1000 * 1000 * 1000,
	"gb":  1000 * 1000 * 1000,
	"kib": 1024,
	"mib": 1024 * 1024,
	"gib": 1024 * 1024 * 1024,
	"tib": 1024 * 1024 * 1024 * 1024,
}

// Parse converts a human-readable byte string into a byte count.
func Parse(input string) (int64, error) {
	raw := strings.TrimSpace(strings.ToLower(input))
	if raw == "" {
		return 0, fmt.Errorf("size cannot be empty")
	}

	var numberPart strings.Builder
	var unitPart strings.Builder

	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' {
			if unitPart.Len() > 0 {
				return 0, fmt.Errorf("invalid size %q", input)
			}
			numberPart.WriteRune(r)
			continue
		}

		unitPart.WriteRune(r)
	}

	value, err := strconv.ParseFloat(numberPart.String(), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", input)
	}

	unit := unitPart.String()
	if unit == "" {
		unit = "b"
	}

	multiplier, ok := multipliers[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported size unit %q", unit)
	}

	return int64(value * float64(multiplier)), nil
}

// Format renders a byte count using binary units.
func Format(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}

	units := []string{"KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f%s", value, unit)
		}
	}

	return fmt.Sprintf("%dB", size)
}
