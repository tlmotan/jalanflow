# Handover — JalanFlow site design, end of session 2026-11-24 (second session)

For the next chat session that works on the website. Read this, then `AGENTS.md`, then `docs/design/project/DESIGN_BRIEF.md`, `docs/CONTENT.md` and `docs/decisions/0013-light-dot-look.md`.

`HANDOVER.md` (also in the repo root) is from 2026-11-22 and is out of date on project status. Its "Environment gotchas" and "How to talk to this user" sections are still worth reading.

## Where things stand

**All six build steps of decision 0013 are built.** The real site in `web/` now has the light dot look that used to exist only as `docs/design/project/light-look-mockup.html`. Both questions 0013 had left open are decided: pale grey basemap, and pale road with ink cars when zoomed in.

Branch `feat/analysis-replay`:

| State | What |
|---|---|
| Committed in `d846822` | Steps 1–3 (tokens, text column, orb, replay, map restyle), the plain list / detail / How this works views, and the earlier session's lanes-and-cars work (decision 0012) |
| **Not committed** | The basemap and zoomed-in choices, step 4 (list and map highlight each other, tap a road), step 5 (phone layout), step 6 (deep links, share preview) |

Commands for Tim to commit the rest (never run these yourself):

```bash
git add web docs/design/project/DESIGN_BRIEF.md docs/decisions/0013-light-dot-look.md
```

```bash
git commit -m "feat(web): basemap and lane colours chosen, steps 4-6 of decision 0013"
```

`git add web` also stages the removal of `web/src/trial.ts`. Check `git status` first: Tim may already have committed.

Checks at the end of the session: `cd web && npm test` (24 pass) and `npm run build` pass. `make test` was not run, because nothing outside `web/` and `docs/` changed.

## What Tim decided this session

| Topic | Decision |
|---|---|
| Earlier session's `web/` work | Finished. Lanes-and-cars kept; the tabbed panel (`panel.ts`) replaced and deleted |
| Lane drops | Show all 87 for now. No distance filter in `web/` |
| Landing sentence | "87 places around KESAS and LDP where more lanes come in than carry on. A local driver confirmed two of them as daily jams." |
| Road names | From the data ("E5 Lebuhraya Shah Alam"). No ref-to-KESAS table in `web/` |
| Markers | Small ink dots, no rank numbers: ringed = confirmed jam, solid = lane drop, hollow = lanes guessed |
| Flow toggle | Kept as an underlined "Hide flow" / "Show flow" link in the result view |
| How this works | Lane-drop notes plus the capacity notes from `manifest.json`. Rewording happens later in Go |
| Basemap | Pale grey, no labels (compared live against none and against place names) |
| Zoomed in | Pale road, ink cars (compared live against ink road, pale cars) |

## How the site is put together

| File | Job |
|---|---|
| `web/index.html` | The text column's markup: one block per view. Share tags |
| `web/src/style.css` | The only place a colour lives (`:root`). Layout, phone rules |
| `web/src/colors.ts` | Reads the CSS variables for the map and canvas. `laneColours()` for the zoomed-in road |
| `web/src/main.ts` | Wires everything: data, text column, map layers, markers, road taps, deep links |
| `web/src/views.ts` | One view at a time, back stack, Escape, focus handling |
| `web/src/orb.ts` | The logo orb. Disorder = 1 − found ÷ max |
| `web/src/replay.ts` | Map side of "Run the analysis". Text side is driven from `main.ts` by callbacks |
| `web/src/copy.ts` | Every sentence built from data. Pure, tested in `web/test/copy.test.mjs` |
| `web/src/links.ts` | Deep-link parsing. Pure, tested in `web/test/links.test.mjs` |
| `web/src/basemap.ts` | The OpenFreeMap style drained to pale greys; falls back to a plain page |
| `web/src/particles.ts`, `lanes.ts`, `routes.ts`, `vehicles.ts`, `track.ts` | Flow dots, lanes and cars (decision 0012) |
| `web/og-card.html` | Dev-only page that draws the share image; the build ignores it |

Things that are easy to get wrong:
- **Map labels in the road-sign face are HTML markers**, not map text layers, because a map text layer can only use the tile server's fonts.
- **The map's look changes with the view** through one function, `styleMap` in `main.ts` (drive opacity, marker quietness, which markers show, phone refit).
- **Results are held back until the analysis has run.** A road's detail says nothing about flow or the bottleneck before then; the share image shows no red either.
- **Deep links use the lane drop's OpenStreetMap id** (`#drop-n33987957`), not its rank, so they survive a data refresh. `design/project/DESIGN_BRIEF.md` §11 has the full table.
- **A double tap on Run** used to skip the replay or zoom the map. Both are blocked for 0.6 s after the start (`SKIP_GUARD_MS`).

## Open items

### For Tim to judge
- **Four route markers.** E1 and E6 also carry more than 3 km of the drive, so they get a hexagon as well as E5 and E11. 0013 named only KESAS and LDP. The rule is `SHIELD_MIN_KM` in `main.ts`.
- **Phone landing fit.** Before the analysis the phone map fits all 87 lane drops, so the drive is small on a narrow screen. Fitting the drive instead makes it bigger but pushes off-drive markers out of view.
- **24 or 87 lane drops** is still open in 0013; it waits on the pipeline field below.

### Not verified
- **A real phone.** Phone layout was checked only at an emulated 375 × 812.
- **The share preview on WhatsApp and X.** They need the live site, and the Pages deploy is paused while the repo is private.
- **The site address.** The share tags name `https://tlmotan.github.io/jalanflow/` (from `ci.yml`). Change them if the site moves.
- **Reduced motion** was tested by faking the setting in the page, not with the system setting.
- **Keyboard focus on a list row** should ring its marker as hover does. Hover was confirmed; focus was not, because the test browser did not deliver focus events.

### Needs the Go pipeline (do not fake in `web/`)
- A common road name ("KESAS", "LDP") beside the ref.
- Something to tell repeated rows apart ("ramp on E11 narrows" appears many times in the list).
- Which lane drops are on the published drive, so the count and headline are true and the landing sentence can be tightened.
- Plain wording for the notes shown in "How this works": "flagged lanes_inferred" and the bracketed class lists still leak through.
- An OSM id on roads, if a tapped road should get an OpenStreetMap link or a deep link.

Any of these changes the output format: update the Go struct, `web/src/types.ts` and ARCHITECTURE.md §4 in the same change (AGENTS.md).

### Small leftovers
- The fixes list is gone; the result shows only the leading fix as a sentence, with its difficulty and reasons. The corridor has one fix today. If more appear, decide where they go.
- Going back from a detail view on desktop leaves the map zoomed in where the detail was.
- `docs/CONTENT.md` was updated in `d846822`; nothing in steps 4–6 changed wording rules.

## About the mockup
`docs/design/project/light-look-mockup.html` is still the reference picture, but the site has moved past it in places (basemap, result link to the list, road taps, "3 → 2" lane counts). Where they differ, `docs/design/project/DESIGN_BRIEF.md` is the source of truth.

## Things tried and rejected
- **Dark version.** Tim: "we should stick to light mode."
- **White floating card, five colours, legend.** Tim: "feels like normal boilerplate design".
- **Tabbed panel** (Jam spots / Lane drops / Fixes). Replaced by one text column.
- **No basemap**, and **basemap with place names.** Tim chose pale grey with no labels.
- **Ink road with pale cars** when zoomed in. Tim chose pale road with ink cars.
- **A setup prompt Tim was sent** (Figma MCP, Playwright MCP, taste skills, Next.js and Tailwind). It did not fit: it would overwrite `DESIGN.md` and break the no-new-dependencies rule. Two ideas from it were kept: test as a separate step and list problems before fixing, and "`DESIGN.md` wins" over any skill.
- Cream paper + serif + terracotta, all-caps labels above headings, facts joined by middle dots, and a giant number with a small caption.

## Working with Tim
- **Never run `git add`, `git commit` or `git merge`.** Hand Tim the commands, scoped to the files from your own session.
- When discussing roads, give copy-paste `lat,lon` and Google Maps links.
- He judges design by seeing it move. Build switchable options and let him compare live (a temporary URL parameter worked well), then delete the losing option.
- He answers quickly when asked one decision at a time with a recommended option first.

## Tools
- Dev server: `.claude/launch.json` has `web` (port 5173) and `pages-preview` (port 4173). This session's server landed on 5174 because 5173 was taken.
- The built-in browser pane can open the site, resize to phone width and take screenshots. In scripted checks, wait for the "Run the analysis" button to appear before clicking: the data arrives a few seconds after the page.
- A page left open through many edits can hold stale code; reload before trusting a failed check.
- To redo the share image, see `design/project/DESIGN_BRIEF.md` §11 (headless Chrome against the dev server).
- Agents in `.claude/agents/`: `web-designer` (works only in `web/`) and `researcher`.
- Checks before calling web work done: `cd web && npm test && npm run build`, then look at it at desktop and 375 px.
