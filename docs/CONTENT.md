# CONTENT — how the site talks

Read with `design/project/DESIGN_BRIEF.md` before changing any text on the site.

## Voice
- Plain, direct, local. Write for a commuter on KESAS, not a traffic engineer.
- Use the names people say: KESAS, LDP, "exit 507", Sri Muda. Never OSM ids or edge ids in visible text (links to OSM are fine).
- Short, active sentences. Second person where natural: "your drive", "you queue here".

## Glossary
| Say | Instead of | Meaning |
|---|---|---|
| bottleneck | min cut | the roads that limit how much can get through |
| the most traffic the roads can carry | max flow | capacity of the network between the two ends |
| veh/h, or "vehicles an hour" in a full sentence | vph, cars/hr | vehicles per hour. "veh/h" in labels, lists and map text; spelled out in the result sentence |
| lane drop | lane reduction, taper | more lanes come in than carry on |
| nearly full | high traffic, busy, congested | the analysis fills the road to 90 % or more of what it can carry. Nothing is measured, so never a word that sounds like live traffic |
| guessed | inferred | lane count not tagged in OSM, taken from a class default |
| fix / +1 lane | intervention | in Phase 1 the only fix type is one extra lane |
| route | augmenting path | one way through the network the algorithm found |

## Caveats
- Say each caveat once, specifically, next to the claim it limits: "This counts lanes, not cars."
- Don't hedge every sentence. If a number is uncertain, say why once and label it ("guessed", "low confidence").

## Avoid
- "AI", "smart", "predicts", "real-time", "live traffic", "solves the jam".
- Blame language about drivers, JKR, concessionaires or anyone else.
- Jargon without a plain gloss (Dinic's, residual graph, CTM) outside "How this works".

## Numbers and dates
- Round veh/h to the nearest 100 on the site; use thousands separators ("6,000 veh/h").
- Lane counts as "3 → 2 lanes".
- Always show the OpenStreetMap data date.

## Labels
- Buttons are verbs: "Run the analysis", "Skip to the result", "Replay", "Back". "Hide flow" / "Show flow" is an underlined link in the result view.
- Links say where they go, with a count where there is one: "See all 87 lane drops", "How this works", "Look at it in Street View". There are no tabs (decision 0013).
- The landing sentence counts every lane drop in the data and says where they are, not that they are on the drive: "87 places around KESAS and LDP where more lanes come in than carry on. A local driver confirmed two of them as daily jams." Tighten it once the pipeline says which drops are on the drive.
- Roads are named from the data ("E5 Lebuhraya Shah Alam") until the pipeline adds a common name ("KESAS"). No ref-to-name table in `web/`.
- The result is one sentence a commuter could repeat: "At most 6,000 vehicles an hour can get from Kota Kemuning to PJ." Then the bottleneck in a second sentence, then the fix.
- No all-caps labels above headings, and no strings of facts joined by dots; write a sentence. One exception: the one- to three-word label at the top of a map card ("CONFIRMED JAM", "BOTTLENECK") is shown in capitals (decision 0015, Tim, 2026-11-24).
- A fix is named by its road, as lane drops are ("E5 Lebuhraya Shah Alam: +1 lane"), then lanes before → after, gain in veh/h, difficulty.
- Captions during the replay are one short sentence each.
