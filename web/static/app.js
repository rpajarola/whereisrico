// Renders the whereisrico GeoJSON feed (trip legs + waypoints) on a
// MapLibre GL globe, replacing the old Google Maps KmlLayer overlay.
//
// The server (internal/geojson) supplies raw facts per feature (trip_index,
// color_index, is_latest_trip, timestamps, is_here); this file is
// responsible for all presentation -- palette and recency-based opacity
// fade (shared with cesium.js via shared.js) plus the "I am here" marker --
// as MapLibre paint expressions, so tweaking the look never requires a
// server change.

const colorExpression = (colorIndexField) => [
  "case",
  ["get", "is_latest_trip"],
  WR_LATEST_TRIP_COLOR,
  [
    "match",
    ["get", colorIndexField],
    0, WR_PALETTE[0],
    1, WR_PALETTE[1],
    2, WR_PALETTE[2],
    3, WR_PALETTE[3],
    4, WR_PALETTE[4],
    WR_PALETTE[0],
  ],
];

function opacityExpression(tsField, minTs, maxTs, minOpacity) {
  if (minTs === maxTs) {
    return 1;
  }
  return [
    "interpolate", ["linear"], ["get", tsField],
    minTs, minOpacity,
    maxTs, 1.0,
  ];
}

function computeBounds(geojson) {
  const bounds = new maplibregl.LngLatBounds();
  let any = false;
  for (const f of geojson.features) {
    const g = f.geometry;
    if (g.type === "Point") {
      bounds.extend(g.coordinates);
      any = true;
    } else if (g.type === "LineString") {
      for (const c of g.coordinates) {
        bounds.extend(c);
        any = true;
      }
    }
  }
  return any ? bounds : null;
}

async function main() {
  const geojson = await wrFetchGeoJSON();
  const { min_timestamp: minTs, max_timestamp: maxTs } = geojson.meta;

  const map = new maplibregl.Map({
    container: "map",
    style: "https://demotiles.maplibre.org/style.json",
    center: [0, 20],
    zoom: 1.2,
    projection: "globe",
  });
  map.addControl(new maplibregl.NavigationControl(), "top-right");

  map.on("load", () => {
    map.addSource("track", { type: "geojson", data: geojson });

    map.addLayer({
      id: "trip-lines",
      type: "line",
      source: "track",
      filter: ["==", ["get", "kind"], "trip"],
      layout: { "line-join": "round", "line-cap": "round" },
      paint: {
        "line-color": colorExpression("color_index"),
        "line-opacity": opacityExpression("end_timestamp", minTs, maxTs, WR_MIN_LINE_OPACITY),
        "line-width": 3,
      },
    });

    map.addLayer({
      id: "waypoints",
      type: "circle",
      source: "track",
      filter: ["all", ["==", ["get", "kind"], "waypoint"], ["!=", ["get", "is_here"], true]],
      paint: {
        "circle-color": colorExpression("color_index"),
        "circle-opacity": opacityExpression("timestamp", minTs, maxTs, WR_MIN_POINT_OPACITY),
        "circle-radius": 5,
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 1,
      },
    });

    map.addLayer({
      id: "i-am-here",
      type: "circle",
      source: "track",
      filter: ["==", ["get", "is_here"], true],
      paint: {
        "circle-color": "#ffffff",
        "circle-radius": 9,
        "circle-stroke-color": WR_LATEST_TRIP_COLOR,
        "circle-stroke-width": 3,
      },
    });

    const bounds = computeBounds(geojson);
    if (bounds) {
      map.fitBounds(bounds, { padding: 40, duration: 0 });
    }

    const popup = new maplibregl.Popup({ closeButton: true, closeOnClick: true });
    for (const layerID of ["waypoints", "i-am-here"]) {
      map.on("click", layerID, (e) => {
        const feature = e.features[0];
        popup
          .setLngLat(feature.geometry.coordinates)
          .setHTML(wrPopupHTML(feature.properties))
          .addTo(map);
      });
      map.on("mouseenter", layerID, () => { map.getCanvas().style.cursor = "pointer"; });
      map.on("mouseleave", layerID, () => { map.getCanvas().style.cursor = ""; });
    }
  });
}

main().catch((err) => {
  console.error("whereisrico:", err);
});
