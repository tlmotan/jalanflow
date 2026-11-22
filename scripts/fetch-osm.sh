#!/usr/bin/env bash
# Download the Malaysia/Singapore/Brunei extract and clip it to a corridor bbox.
# Needs: curl, osmium-tool, jq.   Usage: scripts/fetch-osm.sh federal-highway
set -euo pipefail
CORRIDOR="${1:?corridor id}"
CFG="config/corridors/${CORRIDOR}.json"
mkdir -p data/raw
SRC=data/raw/malaysia-singapore-brunei-latest.osm.pbf
[ -f "$SRC" ] || curl -fL -o "$SRC" https://download.geofabrik.de/asia/malaysia-singapore-brunei-latest.osm.pbf
BBOX=$(jq -r '.bbox | join(",")' "$CFG")
osmium extract --bbox "$BBOX" --strategy complete_ways --overwrite -o "data/raw/${CORRIDOR}.osm.pbf" "$SRC"
echo "wrote data/raw/${CORRIDOR}.osm.pbf"
