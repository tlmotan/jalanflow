# DESKTOP.md — Desktop and web UI rules

> Inherits `DESIGN.md`. Apply to desktop browser and desktop application experiences. For responsive web at phone widths, also apply `MOBILE.md`'s touch and narrow-layout guidance; native-only rules do not apply to browser pages.

## 1. Layout

- Make the primary workspace, content or task visually dominant. Use available width when it improves the task; do not fill empty space merely because it exists.
- A single column is appropriate for focused reading or forms; a contextual side panel is appropriate for inspecting details alongside a canvas or workspace. Use multi-column layouts only when simultaneous visibility matters.
- Use a comfortable reading measure (approximately 55–75 characters per line as a starting point for prose). Do not impose this on maps, tables or code editors.
- Use content-driven breakpoints. Test narrow windows and split-screen layouts as well as wide monitors; do not assume a fixed viewport.
- Avoid horizontal page scrolling. Intentional horizontal scrolling for a table or canvas must be discoverable and operable without hiding key actions.
- Do not push the main application below a marketing hero. Product pages may use a hero when it serves a genuine onboarding or marketing task.

## 2. Desktop composition and sizing

Use shared tokens. The following CSS logical pixel values are optional starting points for web, not fixed requirements:

| Role | Starting range |
|---|---|
| Display heading (where warranted) | 32–48 px |
| Section heading | 20–26 px |
| Body text | 16–18 px |
| Labels / metadata | 12–14 px |
| Main content padding | 16–32 px |
| Contextual side panel | 300–400 px, responsive |

- Keep frequently used controls accessible without crowding the main content.
- Use a compact header or navigation area. Persistent sidebars are justified by frequent navigation, not by aesthetic convention.
- Use alignment and spacing to separate sections before adding boxed panels.
- For a dense workspace, compact controls are acceptable when still accessible; preserve adequate targets and spacing.

## 3. Navigation and input

- Provide a clear current section, meaningful labels and logical navigation order. Use breadcrumbs for genuinely deep hierarchies.
- Hover should reinforce affordances, never be the only path to essential information.
- Support keyboard navigation, visible focus, standard activation and expected Escape dismissal. Manage focus correctly for modal dialogs and return focus to the invoking element when appropriate.
- Preserve native link behaviour for web navigation; use buttons for actions. Respect browser back/forward and meaningful deep links when relevant.
- Support mouse, keyboard, trackpad and touch-enabled desktop devices. Do not assume pointer precision from screen width alone.
- Make controls easy to target. On web, meet applicable WCAG 2.2 target-size requirements (24 × 24 CSS px minimum or a permitted exception); favour larger targets for common controls and coarse pointers.

## 4. Contextual surfaces

Choose the lightest pattern that preserves the task:

- Popover: brief contextual details or a compact menu.
- Side panel: inspection or editing while retaining the main workspace.
- Inline expansion: secondary detail related to the current reading flow.
- Modal: a focused task or decision that genuinely requires interruption.

Avoid stacked dialogs and panels. Preserve selections, scroll position and filters when appropriate. If a panel opens over a map or canvas, keep the relevant feature visible or adjust the viewport to accommodate it. Do not let sticky headers or panels hide focused elements.

## 5. Forms, tables and visualisations

- Use persistent labels, appropriate input types, concise helper text, nearby validation and clear action hierarchy.
- Preserve entries on recoverable errors and support browser autofill when appropriate.
- For tables, use clear column headings, sensible alignment, readable rows and meaningful sorting/filtering only where supported. Do not replace a simple list with an unnecessary data grid.
- Keep map/chart legends and controls compact; prevent annotation collisions. Make selectable features inspectable by keyboard or through an accessible equivalent list.
- Keep meaningful data and methodology accessible without obscuring the primary visualisation.

## 6. Browser and desktop conventions

- Respect browser zoom, text resizing and user preferences; do not disable pinch zoom.
- Support deep links and refresh behaviour where application state warrants it.
- Use native browser or OS controls when they provide better accessibility and familiarity. For installed desktop apps, respect operating system windowing, menus and keyboard shortcuts.
- Avoid hover-dependent menus on touch-enabled laptops; offer explicit activation.
- Prefer stable layouts and responsive loading feedback over disruptive page-wide spinners.

## 7. Desktop acceptance checks

Verify representative viewport widths (e.g. 768, 1024, 1280 and 1440 CSS px for web, plus phone widths if responsive), and at least one short viewport. These are test examples, not mandatory breakpoints.

- The main task is apparent and content is not displaced by decorative UI.
- No unintended overflow, overlapping content or clipped critical labels.
- Navigation, forms, dialogs and controls work with keyboard and pointer.
- Focus is visible and not obscured; browser zoom and text enlargement remain usable.
- Panels preserve the main workspace and can be dismissed.
- Loading, empty, error and selected states are coherent.
- Visualisation legends, labels and data qualifiers remain legible.
- Report which states and widths were actually tested.
