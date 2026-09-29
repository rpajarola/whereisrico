package airport

import "testing"

func TestByIATAKnownCodes(t *testing.T) {
	lookup, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		code      string
		wantCity  string
		latApprox float64
		lonApprox float64
	}{
		{"ZRH", "Zurich", 47.45, 8.56},
		{"SFO", "San Francisco", 37.62, -122.38},
		{"SYD", "Sydney", -33.94, 151.18},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			a, ok := lookup.ByIATA(c.code)
			if !ok {
				t.Fatalf("ByIATA(%q): not found", c.code)
			}
			if a.City != c.wantCity {
				t.Errorf("ByIATA(%q).City = %q, want %q", c.code, a.City, c.wantCity)
			}
			if diff := a.Latitude - c.latApprox; diff < -1 || diff > 1 {
				t.Errorf("ByIATA(%q).Latitude = %v, want ~%v", c.code, a.Latitude, c.latApprox)
			}
			if diff := a.Longitude - c.lonApprox; diff < -1 || diff > 1 {
				t.Errorf("ByIATA(%q).Longitude = %v, want ~%v", c.code, a.Longitude, c.lonApprox)
			}
		})
	}

	// Case-insensitive lookup.
	if _, ok := lookup.ByIATA("zrh"); !ok {
		t.Errorf("ByIATA(\"zrh\"): not found, want case-insensitive match")
	}

	if _, ok := lookup.ByIATA("ZZZ"); ok {
		t.Errorf("ByIATA(\"ZZZ\"): want not found for unknown code")
	}
}
