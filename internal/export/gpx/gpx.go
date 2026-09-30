// Package gpx writes storage.Coord slices out as GPX 1.1 files -- the
// inverse of internal/ingest/gpx.
//
// Coordinates with a message are written as <wpt> entries (matching how
// ingest reads a <wpt>'s <name> as a message); every coordinate, messaged
// or not, is also written into a single <trk> track segment so the full
// path is preserved. Re-importing an exported file reproduces the same
// coordinates: the (timestamp, longitude, latitude) unique constraint means
// a re-ingested track point at the same position as an exported waypoint is
// silently deduplicated, and since waypoints are written first, the
// messaged version wins.
package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

type gpxDoc struct {
	XMLName   xml.Name `xml:"gpx"`
	Version   string   `xml:"version,attr"`
	Creator   string   `xml:"creator,attr"`
	Xmlns     string   `xml:"xmlns,attr"`
	Waypoints []wpt    `xml:"wpt"`
	Track     *trk     `xml:"trk,omitempty"`
}

type wpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time"`
	Name string  `xml:"name"`
}

type trk struct {
	Name     string   `xml:"name,omitempty"`
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

// Write writes coords as a GPX 1.1 document to w. coords need not be
// sorted; the output is written in chronological order regardless of input
// order. name, if non-empty, becomes the <trk>'s <name>.
func Write(w io.Writer, name string, coords []storage.Coord) error {
	sorted := make([]storage.Coord, len(coords))
	copy(sorted, coords)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp.Before(sorted[j].Timestamp) })

	doc := gpxDoc{
		Version: "1.1",
		Creator: "whereisricoctl",
		Xmlns:   "http://www.topografix.com/GPX/1/1",
	}

	var trackPoints []trkpt
	for _, c := range sorted {
		t := c.Timestamp.UTC().Format(time.RFC3339)
		if c.Message != "" {
			doc.Waypoints = append(doc.Waypoints, wpt{Lat: c.Latitude, Lon: c.Longitude, Time: t, Name: c.Message})
		}
		trackPoints = append(trackPoints, trkpt{Lat: c.Latitude, Lon: c.Longitude, Time: t})
	}
	if len(trackPoints) > 0 {
		doc.Track = &trk{Name: name, Segments: []trkseg{{Points: trackPoints}}}
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("gpx: write header: %w", err)
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("gpx: encode: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// WriteFile writes coords to a new GPX file at path, creating or
// truncating it.
func WriteFile(path, name string, coords []storage.Coord) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("gpx: create %s: %w", path, err)
	}
	defer f.Close()
	if err := Write(f, name, coords); err != nil {
		return fmt.Errorf("gpx: write %s: %w", path, err)
	}
	return nil
}
