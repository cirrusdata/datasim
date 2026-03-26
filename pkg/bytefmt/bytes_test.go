package bytefmt

import "testing"

// TestParse verifies supported byte-size inputs.
func TestParse(t *testing.T) {
	t.Parallel()

	cases := map[string]int64{
		"1":      1,
		"1KiB":   1024,
		"2MiB":   2 * 1024 * 1024,
		"1.5GB":  1500000000,
		"0.5gib": 512 * 1024 * 1024,
	}

	for input, want := range cases {
		got, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("Parse(%q) = %d, want %d", input, got, want)
		}
	}
}
