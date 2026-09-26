// Writes style-light.json and style-dark.json for the basemap served at <tilesUrl>, and
// prints every string under a text-font, one per line: build.sh keeps those that name a
// fontstack in basemaps-assets, since some text-font values are expressions.
//
//   node style.mjs <tilesUrl> <lang> <outDir>
import { writeFileSync } from "node:fs";
import { layers, namedFlavor } from "@protomaps/basemaps";

const [url, lang, out] = process.argv.slice(2);
// ODbL: the same string MapView adds, so MapLibre shows it once.
const attribution = '<a href="https://www.openstreetmap.org/copyright">© OpenStreetMap contributors</a>';
const fonts = new Set();
const collect = (v) => (typeof v === "string" ? fonts.add(v) : Array.isArray(v) && v.forEach(collect));

for (const flavor of ["light", "dark"]) {
  const style = {
    version: 8,
    glyphs: `${url}/fonts/{fontstack}/{range}.pbf`,
    sprite: `${url}/sprites/v4/${flavor}`,
    sources: {
      protomaps: { type: "vector", url: `pmtiles://${url}/basemap.pmtiles`, attribution },
    },
    layers: layers("protomaps", namedFlavor(flavor), { lang }),
  };
  style.layers.forEach((layer) => collect(layer.layout?.["text-font"]));
  writeFileSync(`${out}/style-${flavor}.json`, JSON.stringify(style));
}

console.log([...fonts].join("\n"));
