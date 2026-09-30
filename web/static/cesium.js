// Renders the whereisrico GeoJSON feed on a CesiumJS 3D globe -- an
// alternative to the MapLibre globe in app.js, sharing the same
// /api/geojson endpoint and the same palette/recency-fade constants (see
// shared.js) so the two renderers look consistent.
//
// No Cesium ion access token is used: both imagery options are offline or
// public-tile-server based (OpenStreetMap tiles, or Cesium's own bundled
// Natural Earth II imagery -- see setUpImageryToggle), and terrain is a
// plain smooth ellipsoid (no elevation data), so this renderer makes no
// network calls beyond map tiles and our own API -- matching the "no API
// keys" design used throughout this app.

async function main() {
  Cesium.Ion.defaultAccessToken = "";

  // Viewer's older `imageryProvider` constructor option is a no-op in
  // current CesiumJS (it's only consulted to decide whether to suppress
  // the BaseLayerPicker's own default layer, never used to actually build
  // one) -- the base tile layer must be passed as `baseLayer`, wrapping the
  // provider in an ImageryLayer explicitly.
  const osmLayer = new Cesium.ImageryLayer(
    new Cesium.OpenStreetMapImageryProvider({ url: "https://tile.openstreetmap.org/" })
  );

  const viewer = new Cesium.Viewer("cesiumContainer", {
    baseLayer: osmLayer,
    terrainProvider: new Cesium.EllipsoidTerrainProvider(),
    baseLayerPicker: false,
    geocoder: false,
    homeButton: true,
    sceneModePicker: true,
    navigationHelpButton: false,
    animation: false,
    timeline: false,
    fullscreenButton: false,
    selectionIndicator: true,
    infoBox: true,
  });
  viewer.scene.globe.enableLighting = false;

  await setUpImageryToggle(viewer, osmLayer);

  const geojson = await wrFetchGeoJSON();
  const { min_timestamp: minTs, max_timestamp: maxTs } = geojson.meta;

  const dataSource = await Cesium.GeoJsonDataSource.load(geojson, {
    clampToGround: false,
  });
  await viewer.dataSources.add(dataSource);

  for (const entity of dataSource.entities.values) {
    const props = entity.properties;
    const kind = props.kind.getValue();
    const colorHex = wrColorForFeature({
      is_latest_trip: props.is_latest_trip.getValue(),
      color_index: props.color_index.getValue(),
    });
    const color = Cesium.Color.fromCssColorString(colorHex);

    if (kind === "trip") {
      const opacity = wrComputeOpacity(props.end_timestamp.getValue(), minTs, maxTs, WR_MIN_LINE_OPACITY);
      entity.polyline.material = color.withAlpha(opacity);
      entity.polyline.width = 3;
      entity.polyline.clampToGround = false;
      continue;
    }

    if (kind !== "waypoint") {
      continue;
    }

    // GeoJsonDataSource defaults every Point feature to a pin-shaped
    // billboard marker; we render our own styled point instead.
    entity.billboard = undefined;

    const isHere = !!(props.is_here && props.is_here.getValue());
    entity.point = new Cesium.PointGraphics({
      pixelSize: isHere ? 14 : 8,
      color: isHere
        ? Cesium.Color.WHITE
        : color.withAlpha(wrComputeOpacity(props.timestamp.getValue(), minTs, maxTs, WR_MIN_POINT_OPACITY)),
      outlineColor: isHere ? Cesium.Color.fromCssColorString(WR_LATEST_TRIP_COLOR) : Cesium.Color.WHITE,
      outlineWidth: isHere ? 3 : 1,
      // disableDepthTestDistance defaults to 0 (always depth-test), which
      // is what we want: a point on the far side of the globe should be
      // occluded by it, not drawn through it.
    });
    entity.description = wrPopupHTML({
      message: props.message.getValue(),
      timestamp: props.timestamp.getValue(),
    });
  }

  await viewer.zoomTo(dataSource);
}

// setUpImageryToggle adds a second base imagery option -- Natural Earth II
// low-resolution world imagery, vendored into web/static/imagery/ (see
// ATTRIBUTION.md there) and served from this app's own origin rather than
// Cesium's CDN, so it has no external dependency at all -- and a button to
// switch between it and osmLayer. Unlike OpenStreetMapImageryProvider,
// TileMapServiceImageryProvider has no synchronous constructor that takes
// just a URL: it has to fetch and parse the tileset's tilemapresource.xml
// first to know its bounds/zoom levels, so building it is async.
async function setUpImageryToggle(viewer, osmLayer) {
  const naturalEarthProvider = await Cesium.TileMapServiceImageryProvider.fromUrl(
    "imagery/NaturalEarthII"
  );
  const naturalEarthLayer = viewer.imageryLayers.addImageryProvider(naturalEarthProvider);
  naturalEarthLayer.show = false;

  const labels = {
    osm: "Imagery: OpenStreetMap",
    naturalEarth: "Imagery: Natural Earth II (offline)",
  };

  const button = document.createElement("button");
  button.className = "renderer-switch imagery-toggle";
  button.textContent = labels.osm;

  let usingOSM = true;
  button.addEventListener("click", () => {
    usingOSM = !usingOSM;
    osmLayer.show = usingOSM;
    naturalEarthLayer.show = !usingOSM;
    button.textContent = usingOSM ? labels.osm : labels.naturalEarth;
  });

  document.body.appendChild(button);
}

main().catch((err) => {
  console.error("whereisrico (cesium):", err);
});
