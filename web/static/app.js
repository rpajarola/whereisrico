// Renders the whereisrico GeoJSON feed (trip legs + waypoints) on a
// MapLibre GL globe, replacing the old Google Maps KmlLayer overlay.
//
// The server (internal/geojson) supplies raw facts per feature (trip_index,
// color_index, is_latest_trip, timestamps, is_here); this file is
// responsible for all presentation -- palette, recency-based opacity fade,
// and the "I am here" marker -- as MapLibre paint expressions, so tweaking
// the look never requires a server change.

const PALETTE = [
  "#3366ff", // blue
  "#8a2be2", // blueviolet
  "#ff7f50", // coral
  "#dc143c", // crimson
  "#00b7c2", // cyan (darkened slightly for contrast on light basemaps)
];
const LATEST_TRIP_COLOR = "#e00000";
const MIN_LINE_OPACITY = 0.3;
const MIN_POINT_OPACITY = 0.35;

const colorExpression = (colorIndexField) => [
  "case",
  ["get", "is_latest_trip"],
  LATEST_TRIP_COLOR,
  [
    "match",
    ["get", colorIndexField],
    0, PALETTE[0],
    1, PALETTE[1],
    2, PALETTE[2],
    3, PALETTE[3],
    4, PALETTE[4],
    PALETTE[0],
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

function formatTimestamp(unixSeconds) {
  return new Date(unixSeconds * 1000).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function popupHTML(props) {
  return `<div class="message">${escapeHTML(props.message)}</div>` +
    `<div class="date">${formatTimestamp(props.timestamp)}</div>`;
}

function escapeHTML(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

async function main() {
  const resp = await fetch("/api/geojson");
  if (!resp.ok) {
    throw new Error(`fetching /api/geojson: ${resp.status}`);
  }
  const geojson = await resp.json();
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
        "line-opacity": opacityExpression("end_timestamp", minTs, maxTs, MIN_LINE_OPACITY),
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
        "circle-opacity": opacityExpression("timestamp", minTs, maxTs, MIN_POINT_OPACITY),
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
        "circle-stroke-color": LATEST_TRIP_COLOR,
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
          .setHTML(popupHTML(feature.properties))
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
