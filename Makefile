CORRIDOR ?= kemuning-pj

# paulmach/osm/osmpbf links C zlib (needs pkg-config) when cgo is on.
# The pure-Go zlib path needs no system libraries.
export CGO_ENABLED ?= 0

.PHONY: fetch graph analyze calibrate test web all clean hooks shots

fetch:
	scripts/fetch-osm.sh $(CORRIDOR)

graph:
	go run ./cmd/build-graph -in data/raw/$(CORRIDOR).osm.pbf -out data/interim/$(CORRIDOR).graph.json

analyze:
	go run ./cmd/analyze -graph data/interim/$(CORRIDOR).graph.json \
		-corridor config/corridors/$(CORRIDOR).json -out web/public/data/$(CORRIDOR)

# Estimate uncounted traffic by fitting the simulation to observed travel
# times; rewrites config/demand/$(CORRIDOR).json (decision 0009). Run by hand
# when observations change; not part of `make all`.
calibrate:
	go run ./cmd/calibrate -graph data/interim/$(CORRIDOR).graph.json \
		-corridor config/corridors/$(CORRIDOR).json \
		-demand config/demand/$(CORRIDOR).json -observed config/observed/$(CORRIDOR).json

test:
	go vet ./...
	go test ./...

web:
	cd web && npm ci && npm run build

all: fetch graph analyze test web

# Screenshot the user flow (desktop, phone, short phone) into screenshots/flow,
# for comparing with the approved design (docs/design/WORKFLOW.md step 7).
# Uses the dev server at SHOTS_URL if it is up, otherwise starts one. Not part of `make all`.
SHOTS_URL ?= http://localhost:5173
shots:
	scripts/shots.sh $(SHOTS_URL)

# Run make test / npm run build before each commit (scripts/git-hooks/pre-commit).
hooks:
	git config core.hooksPath scripts/git-hooks
	@echo "pre-commit hook enabled"

clean:
	rm -rf data/interim/* web/dist
