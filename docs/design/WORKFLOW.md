# WORKFLOW.md — from design guides to verified UI

> The order every UI change follows. Tool-specific steps live in `integrations/` (today: `integrations/OPENPENCIL.md`). Project rules live in `project/DESIGN_BRIEF.md`.

## Files

```
docs/design/
├── DESIGN.md            shared foundation, any platform
├── DESKTOP.md           desktop and web layouts
├── MOBILE.md            phone and touch layouts
├── WORKFLOW.md          this file
├── integrations/
│   └── OPENPENCIL.md    how agents drive the design canvas
└── project/
    ├── DESIGN_BRIEF.md  JalanFlow's own rules; wins where it is more specific
    ├── design.fig       the editable design document (OpenPencil)
    └── light-look-mockup.html   the light dot look reference (decision 0013)
```

Wording rules are in `docs/CONTENT.md`; design decisions with a story are ADRs in `docs/decisions/`.

## The steps

1. **Read the guides.** `DESIGN.md`, then `DESKTOP.md` and/or `MOBILE.md` (both for the responsive site), then `project/DESIGN_BRIEF.md` and `docs/CONTENT.md`, then the ADRs the brief names. The brief wins where it is more specific.
2. **Read the product.** The current code under `web/`, its tokens, and the published data in `web/public/data/`. Numbers on a design come from the data, never from a document or an earlier mock-up.
3. **Design on the canvas.** Build or update `project/design.fig` through the design tool's MCP server (see `integrations/`). A design pass delivers: the current screen as a baseline, the proposed screens, every interaction state the code supports, and the components and tokens they use. Token changes are listed explicitly, never slipped in.
4. **Check the design.** Run the tool's lint, apply the safe fixes, and fix or explain what is left. Export a PNG of each frame to `screenshots/<tool>/` for review.
5. **Human design review (gate).** Tim approves, asks for changes, or rejects. Nothing under `web/` changes before an approval in chat. Approval of one frame is not approval of the rest. A decision with a story becomes an ADR.
6. **Implement.** Change `web/` to match the approved frames, reusing the existing tokens and components. AGENTS.md rules still hold: no new dependencies, plain DOM, `strict` TypeScript, colours only in CSS `:root`.
7. **Verify in the browser.** Run the dev server, capture the same screens and states (`screenshots/flow.mjs` walks the site), and compare them with the approved PNGs. Then run the acceptance checks: `DESKTOP.md` §7, `MOBILE.md` §9, `project/DESIGN_BRIEF.md` §12. Report every difference, and every width or state not checked.

## Rules for every step

- **Truthful.** A design never shows data the model did not compute, and observed, computed and tested stay apart (`project/DESIGN_BRIEF.md` §3).
- **Small.** Prefer the smallest change that solves the problem; a redesign needs asking first.
- **Written down.** If a step changes a rule, update the guide it belongs to in the same change.
