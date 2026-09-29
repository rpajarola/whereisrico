package geojson

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON renders {"type": ..., "coordinates": ...} per the GeoJSON
// spec, picking Point or LineString based on Type.
func (g Geometry) MarshalJSON() ([]byte, error) {
	switch g.Type {
	case "Point":
		return json.Marshal(struct {
			Type        string     `json:"type"`
			Coordinates [2]float64 `json:"coordinates"`
		}{g.Type, g.Point})
	case "LineString":
		return json.Marshal(struct {
			Type        string       `json:"type"`
			Coordinates [][2]float64 `json:"coordinates"`
		}{g.Type, g.LineString})
	default:
		return nil, fmt.Errorf("geojson: unknown geometry type %q", g.Type)
	}
}

// UnmarshalJSON is provided so tests (and any future consumer) can round
// trip a Geometry.
func (g *Geometry) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	g.Type = raw.Type
	switch raw.Type {
	case "Point":
		return json.Unmarshal(raw.Coordinates, &g.Point)
	case "LineString":
		return json.Unmarshal(raw.Coordinates, &g.LineString)
	default:
		return fmt.Errorf("geojson: unknown geometry type %q", raw.Type)
	}
}
