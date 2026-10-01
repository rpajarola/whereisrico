// Constants and pure helpers shared by every whereisrico frontend renderer
// (app.js for MapLibre, cesium.js for CesiumJS). Keeping the palette and
// recency-fade math in one place means the two renderers can't visually
// drift apart from each other.
//
// As with the server (internal/geojson), presentation lives entirely in the
// frontend: these helpers turn the raw facts a feature carries (color_index,
// is_latest_trip, timestamps) into concrete colors/opacities.

const WR_PALETTE = [
  "#3366ff", // blue
  "#8a2be2", // blueviolet
  "#ff7f50", // coral
  "#dc143c", // crimson
  "#00b7c2", // cyan (darkened slightly for contrast on light basemaps)
];
const WR_LATEST_TRIP_COLOR = "#e00000";
const WR_MIN_LINE_OPACITY = 0.3;
const WR_MIN_POINT_OPACITY = 0.35;

function wrColorForFeature(props) {
  if (props.is_latest_trip) {
    return WR_LATEST_TRIP_COLOR;
  }
  return WR_PALETTE[props.color_index % WR_PALETTE.length];
}

// Linearly interpolates opacity from minOpacity (oldest, ts === minTs) to
// 1.0 (most recent, ts === maxTs), matching the old KML's recency fade.
function wrComputeOpacity(ts, minTs, maxTs, minOpacity) {
  if (minTs === maxTs) {
    return 1;
  }
  const t = (ts - minTs) / (maxTs - minTs);
  return minOpacity + Math.max(0, Math.min(1, t)) * (1 - minOpacity);
}

async function wrFetchGeoJSON() {
  // Relative, not "/api/geojson": this frontend may be reverse-proxied
  // under a path prefix (e.g. Apache ProxyPass "/whereisrico/" -> this
  // server's "/"), and every other same-origin reference in this app
  // (script/link tags, cesium.js's imagery URL) is already relative for
  // exactly that reason. An absolute path here would resolve against the
  // site root instead of the page's own directory, bypassing the prefix
  // entirely and 404ing behind such a proxy.
  const resp = await fetch("api/geojson");
  if (!resp.ok) {
    throw new Error(`fetching api/geojson: ${resp.status}`);
  }
  return resp.json();
}

function wrFormatTimestamp(unixSeconds) {
  return new Date(unixSeconds * 1000).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function wrEscapeHTML(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

function wrPopupHTML(props) {
  return `<div class="message">${wrEscapeHTML(props.message)}</div>` +
    `<div class="date">${wrFormatTimestamp(props.timestamp)}</div>`;
}
