package googletimeline

import (
	"strings"
	"testing"
	"time"
)

const sample = `{
  "semanticSegments": [
    {
      "startTime": "2013-08-31T16:00:00.000-04:00",
      "endTime": "2013-08-31T18:00:00.000-04:00",
      "timelinePath": [
        {"point": "46.689618°, 7.7717974°", "time": "2013-08-31T16:08:00.000-04:00"},
        {"point": "46.689618°, 7.7717974°", "time": "2013-08-31T16:08:00.000-04:00"}
      ]
    },
    {
      "startTime": "2013-08-31T16:08:15.000-04:00",
      "endTime": "2013-08-31T17:20:35.000-04:00",
      "visit": {"topCandidate": {"placeLocation": {"latLng": "46.6896731°, 7.7733948°"}}}
    },
    {
      "startTime": "2013-09-13T05:11:11.000-04:00",
      "endTime": "2013-09-15T05:23:28.000-04:00",
      "activity": {
        "start": {"latLng": "47.4162754°, 8.537611°"},
        "end": {"latLng": "47.4169133°, -8.5371287°"}
      }
    },
    {
      "startTime": "2016-10-21T13:49:33.000-04:00",
      "endTime": "2016-10-24T16:27:00.000-04:00",
      "timelineMemory": {"trip": {"distanceFromOriginKms": 2539}}
    }
  ],
  "rawSignals": [
    {"position": {"LatLng": "-33.8701823°, 151.2054688°", "accuracyMeters": 100, "timestamp": "2026-08-26T06:17:31.000-04:00"}},
    {"position": {"LatLng": "-33.87°, 151.2°", "accuracyMeters": 5000, "timestamp": "2026-08-26T06:18:31.000-04:00"}},
    {"wifiScan": {"deliveryTime": "2026-08-26T06:17:31.000-04:00"}},
    {"activityRecord": {"timestamp": "2026-08-26T06:27:28.000-04:00"}}
  ],
  "userLocationProfile": {"frequentPlaces": [{"placeLocation": "43.49°, -80.47°"}]}
}`

func TestParse(t *testing.T) {
	coords, err := Parse(strings.NewReader(sample), Options{IncludeRaw: true, MaxAccuracyMeters: 1000})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// 1 path point (duplicate dropped) + 2 visit + 2 activity + 1 raw (the
	// 5000m fix is filtered out).
	if len(coords) != 6 {
		t.Fatalf("got %d coords, want 6: %+v", len(coords), coords)
	}

	first := coords[0]
	if first.Latitude != 46.689618 || first.Longitude != 7.7717974 {
		t.Errorf("first coord = (%v, %v), want (46.689618, 7.7717974)", first.Latitude, first.Longitude)
	}
	if want := time.Date(2013, 8, 31, 20, 8, 0, 0, time.UTC); !first.Timestamp.Equal(want) {
		t.Errorf("first timestamp = %v, want %v", first.Timestamp, want)
	}
	if first.Source != Source {
		t.Errorf("source = %q, want %q", first.Source, Source)
	}
	if got := coords[4].Longitude; got != -8.5371287 {
		t.Errorf("activity end longitude = %v, want -8.5371287", got)
	}
	if got := coords[5].Latitude; got != -33.8701823 {
		t.Errorf("raw latitude = %v, want -33.8701823", got)
	}
}

func TestParseWithoutRaw(t *testing.T) {
	coords, err := Parse(strings.NewReader(sample), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(coords) != 5 {
		t.Fatalf("got %d coords, want 5", len(coords))
	}
}

func TestParseTimeRange(t *testing.T) {
	coords, err := Parse(strings.NewReader(sample), Options{
		IncludeRaw: true,
		From:       time.Date(2013, 9, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Only the activity's start and end fall in range.
	if len(coords) != 2 {
		t.Fatalf("got %d coords, want 2: %+v", len(coords), coords)
	}
}

func TestParseLatLngErrors(t *testing.T) {
	for _, s := range []string{"46.1°", "abc°, 7°", "46°, x°", "91°, 0°", "0°, 181°"} {
		if _, _, err := parseLatLng(s); err == nil {
			t.Errorf("parseLatLng(%q): want error", s)
		}
	}
}
