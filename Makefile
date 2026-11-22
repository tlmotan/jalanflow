CORRIDOR ?= kemuning-pj

# paulmach/osm/osmpbf links C zlib (needs pkg-config) when cgo is on.
# The pure-Go zlib path needs no system libraries.
export CGO_ENABLED ?= 0

.PHONY: fetch graph analyze test web all clean

fetch:
	scripts/fetch-osm.sh $(CORRIDOR)

graph:
	go run ./cmd/build-graph -in data/raw/$(CORRIDOR).osm.pbf -out data/interim/$(CORRIDOR).graph.json

analyze:
	go run ./cmd/analyze -graph data/interim/$(CORRIDOR).graph.json \
		-corridor config/corridors/$(CORRIDOR).json -out web/public/data/$(CORRIDOR)

test:
	go vet ./...
	go test ./...

web:
	cd web && npm ci && npm run build

all: fetch graph analyze test web

clean:
	rm -rf data/interim/* web/dist
