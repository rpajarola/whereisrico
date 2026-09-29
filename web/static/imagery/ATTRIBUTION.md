# Data attribution

`NaturalEarthII/` is the low-resolution world imagery tileset bundled with
CesiumJS (`Build/Cesium/Assets/Textures/NaturalEarthII` in the `cesium` npm
package, version 1.145.0), vendored here so the Cesium 3D globe renderer
(`web/static/cesium.js`) can serve it from this app's own origin instead of
depending on the unpkg CDN at runtime.

Per Cesium's own `LICENSE.md`, this is public domain data from Natural
Earth (naturalearthdata.com); no attribution is legally required, though
Natural Earth appreciates a credit.
