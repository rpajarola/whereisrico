package thin

import (
	"math"
	"testing"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

var t0 = time.Date(2024, 6, 26, 0, 0, 0, 0, time.UTC)

// pt returns a coord minutes after t0, km kilometers north of (0, 0).
func pt(minutes int, km float64) storage.Coord {
	return storage.Coord{Timestamp: t0.Add(time.Duration(minutes) * time.Minute), Latitude: km / 111.195}
}

var opts = Options{Interval: 20 * time.Minute, MaxDistanceMeters: 20000, MinDistanceMeters: 500}

func minutesOf(cs []storage.Coord) []int {
	var out []int
	for _, c := range cs {
		out = append(out, int(c.Timestamp.Sub(t0)/time.Minute))
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestThinWalkingKeepsByInterval(t *testing.T) {
	// Walking 100m/minute, one point per minute for an hour.
	var in []storage.Coord
	for m := 0; m <= 60; m++ {
		in = append(in, pt(m, float64(m)*0.1))
	}
	got := minutesOf(Thin(in, opts, nil))
	if want := []int{0, 20, 40, 60}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThinFastTravelKeepsByDistance(t *testing.T) {
	// Train at 6km/minute: 18km after 3 minutes, 24km after 4.
	var in []storage.Coord
	for m := 0; m <= 12; m++ {
		in = append(in, pt(m, float64(m)*6))
	}
	got := minutesOf(Thin(in, opts, nil))
	if want := []int{0, 4, 8, 12}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThinStationaryCollapses(t *testing.T) {
	// Six hours jittering within 100m, then a move.
	var in []storage.Coord
	for m := 0; m < 360; m += 5 {
		in = append(in, pt(m, 0.1*float64(m%2)))
	}
	in = append(in, pt(400, 3))
	got := minutesOf(Thin(in, opts, nil))
	if want := []int{0, 400}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThinExcludesWindowInterior(t *testing.T) {
	in := []storage.Coord{pt(0, 0), pt(30, 100), pt(60, 200), pt(90, 300), pt(120, 400)}
	w := []Window{{Start: t0.Add(30 * time.Minute), End: t0.Add(90 * time.Minute)}}
	got := minutesOf(Thin(in, opts, w))
	if want := []int{0, 30, 90, 120}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThinUnsortedAndEmpty(t *testing.T) {
	if got := Thin(nil, opts, nil); got != nil {
		t.Errorf("Thin(nil) = %v, want nil", got)
	}
	got := minutesOf(Thin([]storage.Coord{pt(40, 2), pt(0, 0), pt(20, 1)}, opts, nil))
	if want := []int{0, 20, 40}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThinDropsSpikes(t *testing.T) {
	o := opts
	o.MaxSpeedKmh = 1200
	// A stray fix 10,000km away for one sample, then back on track.
	in := []storage.Coord{pt(0, 0), pt(20, 2), pt(22, 10000), pt(40, 4), pt(60, 6)}
	got := minutesOf(Thin(in, o, nil))
	if want := []int{0, 20, 40, 60}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// A real long jump (a flight) is kept: only the way in is fast, the
	// way out is ordinary travel at the destination.
	in = []storage.Coord{pt(0, 0), pt(60, 10000), pt(80, 10002)}
	got = minutesOf(Thin(in, o, nil))
	if want := []int{0, 60, 80}; !equalInts(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDistanceMeters(t *testing.T) {
	// Zurich airport to Sydney airport is roughly 16,560 km.
	d := DistanceMeters(47.4647, 8.5492, -33.9461, 151.1772)
	if math.Abs(d-16560000) > 50000 {
		t.Errorf("ZRH-SYD = %.0fm, want ~16560km", d)
	}
}
