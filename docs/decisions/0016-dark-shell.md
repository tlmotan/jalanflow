# 0016 — The dark shell is the site

- Status: accepted · 2026-11-25 (Tim: "i like the dark shell, can you make it the main design").
- Context: Decision 0013 locked in a light page with the text straight on it, "for now". On 2026-11-24 Tim brought a shared design system (`design/DESIGN.md`, `DESKTOP.md`, `MOBILE.md`) and a dark mockup, and a trial page (`web/trial.html`) was built from them beside the site and refined over a day: the run, the flow dots, cars in lanes, the direction switch, map cards (decision 0015), one token system. Tim chose it over the light site.
- Decision:
  - **The trial page is now `web/index.html`.** Its script and stylesheet are `web/src/main.ts`, `web/src/style.css` and `web/src/flow.ts`. The light site's page, `orb.ts`, `particles.ts`, `replay.ts` and `views.ts` are removed; they are in git history at `998132e`.
  - **A dark shell around a light map.** Bar, map card and panel; the panel beside the map on desktop and a sheet below it on phones. Tokens and layout are in `design/project/DESIGN_BRIEF.md` §1, §4 and §5.
  - **The capacity is the one big number on the result**, where 0013 had a sentence. Nothing computed shows before "Run the analysis" (decisions 0004, 0011 unchanged).
  - **A direction switch**, since the data holds both directions.
  - **Map cards are adopted** (decision 0015 is now accepted).
  - **Links name the direction** (`#outbound/result`). Links shared from the light site still open (`web/src/links.ts`, tested).
- What still holds from 0013 and 0014: the light map, the ink markers by shape, red for jams and yellow for nearly full, the route markers, lanes and cars when zoomed in (0012).
- **The lane strip** was carried over the same day (Tim: "Yes, add it"): on the result, above the tested lane, now drawn for the direction shown (`design/project/DESIGN_BRIEF.md` §13).
- Not carried over, to be decided:
  - **The orb** logo. The bar has the name in words.
  - **The share image** (`web/public/og.png`, `web/og-card.html`) still shows the light look.
- Consequences: web only. No output format change, no new dependency.
