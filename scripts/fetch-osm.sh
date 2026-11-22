#!/usr/bin/env bash
# Download OSM roads for a corridor bbox into data/raw/<corridor>.osm.pbf.
# Needs: curl, osmium-tool, jq.   Usage: scripts/fetch-osm.sh kemuning-pj
#
# OSM_SOURCE=overpass (default): ask the Overpass API for just the included
#   road classes in the bbox (a few MB). Classes come from the keys of
#   config/capacity-defaults.json, so there is one list of included roads.
# OSM_SOURCE=geofabrik: download the whole Malaysia/Singapore/Brunei extract
#   (~250 MB) and clip it with osmium. Override OSM_URL to pin a dated file
#   when Geofabrik's "latest" alias is mid-update and 404s.
set -euo pipefail
CORRIDOR="${1:?corridor id}"
CFG="config/corridors/${CORRIDOR}.json"
CAP="config/capacity-defaults.json"
OUT="data/raw/${CORRIDOR}.osm.pbf"
OSM_SOURCE="${OSM_SOURCE:-overpass}"
mkdir -p data/raw

case "$OSM_SOURCE" in
overpass)
  OVERPASS_URL="${OVERPASS_URL:-https://overpass-api.de/api/interpreter}"
  # Config bbox is minLon,minLat,maxLon,maxLat; Overpass wants south,west,north,east.
  BBOX=$(jq -r '.bbox | "\(.[1]),\(.[0]),\(.[3]),\(.[2])"' "$CFG")
  CLASSES=$(jq -r '.classes | keys | join("|")' "$CAP")
  # (._;>;) adds every node of the matched ways, even outside the bbox:
  # the same as osmium's complete_ways, which osm.ReadPBF relies on.
  QUERY="[out:xml][timeout:180];way[\"highway\"~\"^(${CLASSES})(_link)?\$\"](${BBOX});(._;>;);out body;"
  XML="data/raw/${CORRIDOR}.osm"
  # The default curl User-Agent gets HTTP 406 from overpass-api.de.
  curl -fL --compressed --retry 5 --retry-all-errors --retry-delay 10 \
    -A "jalanflow/0.1 (road capacity research)" \
    --data-urlencode "data=${QUERY}" "$OVERPASS_URL" -o "$XML.part"
  mv "$XML.part" "$XML"
  # Carry Overpass's data timestamp into the PBF header so the graph gets an osm_date.
  TS=$(sed -n 's/.*<meta osm_base="\([^"]*\)".*/\1/p' "$XML" | head -1)
  [ -n "$TS" ] || { echo "no osm_base timestamp in Overpass reply" >&2; exit 1; }
  osmium cat "$XML" --output-header="osmosis_replication_timestamp=${TS}" --overwrite -o "$OUT"
  ;;
geofabrik)
  SRC=data/raw/malaysia-singapore-brunei-latest.osm.pbf
  OSM_URL="${OSM_URL:-https://download.geofabrik.de/asia/malaysia-singapore-brunei/malaysia-singapore-brunei-latest.osm.pbf}"
  if [ ! -f "$SRC" ]; then
    curl -fL -o "$SRC.part" "$OSM_URL"
    mv "$SRC.part" "$SRC"
  fi
  BBOX=$(jq -r '.bbox | join(",")' "$CFG")
  osmium extract --bbox "$BBOX" --strategy complete_ways --overwrite -o "$OUT" "$SRC"
  ;;
*)
  echo "OSM_SOURCE must be overpass or geofabrik, got: $OSM_SOURCE" >&2
  exit 2
  ;;
esac
echo "wrote $OUT"
