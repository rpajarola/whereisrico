// Package gpx parses GPX 1.0/1.1 files into a flat list of timestamped
// points, using encoding/xml struct unmarshaling rather than hand-rolled DOM
// walking.
package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"time"
)

// Point is a single waypoint or track point extracted from a GPX file.
type Point struct {
	Time      time.Time
	Latitude  float64
	Longitude float64
	Message   string // <name> on a waypoint; empty for track points.
}

type gpxDoc struct {
	XMLName   xml.Name `xml:"gpx"`
	Waypoints []wpt    `xml:"wpt"`
	Tracks    []trk    `xml:"trk"`
}

type wpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time"`
	Name string  `xml:"name"`
}

type trk struct {
	Segments []trkseg `xml:"trkseg"`
}

type trkseg struct {
	Points []trkpt `xml:"trkpt"`
}

type trkpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time"`
}

// ParseFile parses the GPX file at path.
func ParseFile(path string) ([]Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gpx: open %s: %w", path, err)
	}
	defer f.Close()
	pts, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("gpx: parse %s: %w", path, err)
	}
	return pts, nil
}

// Parse parses GPX content from r.
func Parse(r io.Reader) ([]Point, error) {
	var doc gpxDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("gpx: decode: %w", err)
	}

	var pts []Point
	for _, w := range doc.Waypoints {
		t, err := parseTime(w.Time)
		if err != nil {
			return nil, fmt.Errorf("gpx: waypoint %q: %w", w.Name, err)
		}
		pts = append(pts, Point{Time: t, Latitude: w.Lat, Longitude: w.Lon, Message: w.Name})
	}
	for _, tk := range doc.Tracks {
		for _, seg := range tk.Segments {
			for _, p := range seg.Points {
				t, err := parseTime(p.Time)
				if err != nil {
					return nil, fmt.Errorf("gpx: track point: %w", err)
				}
				pts = append(pts, Point{Time: t, Latitude: p.Lat, Longitude: p.Lon})
			}
		}
	}
	return pts, nil
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("missing <time>")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.UTC(), nil
}
