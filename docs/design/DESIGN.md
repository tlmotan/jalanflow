# DESIGN.md — Shared product design foundation

> T3-inspired product polish. Platform-native usability. Content-first clarity. No unnecessary clutter.

## 0. Instructions for coding agents

Read this file before designing or changing any UI. Also read `DESKTOP.md` for desktop and browser applications, `MOBILE.md` for phone experiences (including mobile web), and any project-specific design brief. For responsive web, apply both platform guides as appropriate to the viewport and input method.

- MUST rules are acceptance requirements; SHOULD rules are defaults that may be adapted for a concrete user need.
- Explicit project requirements determine domain semantics and branding. Platform accessibility and interaction conventions determine implementation. This file supplies the shared design intent. If requirements conflict, document the reason for the chosen solution; never silently change product behaviour or data to improve appearance.
- Inspect the existing UI, tokens, component library and application states before editing. Reuse working patterns. Do not introduce a new framework, dependency, theme or component abstraction solely for cosmetic consistency.
- Prefer a small, coherent change over a wholesale redesign unless requested.

## 1. Design philosophy

Build a distinctive, useful product, not a generic SaaS template. Use [T3 Code](https://t3.codes/) as a reference for confident product identity, typographic hierarchy, technical character and interaction polish, not as a layout or branding template.

- **Content first:** the main task, content, or finding is the centre of the experience. A map product is map-first; a writing product is writing-first; a workflow product prioritises its workflow.
- **One dominant focal point:** establish the most important action or finding on each screen. Secondary actions must remain discoverable without competing for attention.
- **Progressive disclosure:** expose essential information immediately; disclose advanced controls and details when requested.
- **Restrained confidence:** use typography, alignment, negative space, contrast and precise details rather than decorative noise.
- **Platform-native usability:** maintain a recognisable identity while adapting layout, navigation and interactions to the device.
- **Truthfulness:** show what the product actually knows or does; never substitute attractive presentation for accurate data or meaningful feedback.

The intended feel is modern, precise, approachable, quietly technical and deliberate. Avoid imitation, trend-driven embellishment and sterile uniformity.

## 2. Visual system

### Colour and surfaces

MUST use semantic tokens for canvas, surface, raised surface, text, muted text, border, accent, focus, selection and status colours. Map these tokens to project-specific values in one source of truth. Platform files must not independently redefine the palette.

A possible neutral starting palette (not mandatory and not an accessibility-certified combination):

| Role | Example |
|---|---|
| Dark canvas / brand surface | `#111318` |
| Secondary dark surface | `#1C2028` |
| Light canvas | `#F5F6F7` |
| Light border | `#DDE1E5` |
| Primary text on light | `#17191D` |
| Secondary text on light | `#626A73` |
| Primary text on dark | `#FFFFFF` |
| Secondary text on dark | `#A1A7B0` |

- Use a restrained accent; reserve domain and status colours for their established meanings.
- Do not use colour alone to distinguish states. Combine it with shape, text or another accessible cue.
- Dark surfaces may support brand identity; light surfaces may serve dense reading or visualisation. Do not force a dark theme onto content that reads better on light.
- Prefer flat surfaces and subtle borders. Use gradients, shadows and elevation only when justified by hierarchy, layering or a defined brand requirement.
- If multiple themes exist, maintain semantic equivalence and test each theme independently.

### Typography

- Use one coherent primary type family. Add a complementary monospace face only for technical identifiers, code or data where useful.
- Establish roles for display, heading, body, label, metadata and numerical values. Make size and weight differences intentional; avoid bolding everything.
- Use strong display typography sparingly. A large metric or headline should communicate importance, not consume space needed for the primary task.
- Use tabular numerals for changing or comparable values where supported. Keep units with numbers.
- Use comfortable line lengths, line heights and accessible text sizes. Allow wrapping and text scaling instead of clipping essential content.

### Spacing, shape and depth

- Adopt a coherent spacing scale; a 4-unit base with 4/8/12/16/24/32/48/64 increments is a reasonable starting point. Use platform-appropriate logical units.
- Establish spacing between groups before adding containers; align related elements to shared edges.
- Use a small radius scale. Reserve full pills for controls or statuses where the shape communicates meaning.
- Avoid card-within-card layouts, ornamental badges, redundant borders and shadows on every surface.
- Use a consistent icon family. Do not replace unfamiliar actions with ambiguous icons alone.

## 3. Layout and content

- Make the screen purpose and primary action or finding immediately apparent.
- Use the simplest structure that fits the task. Do not default to a sidebar, card grid, large hero or multi-column dashboard.
- Keep supporting details adjacent to the content they explain, or one clear interaction away.
- Preserve the user's context when showing details, changing filters or dismissing panels.
- Use plain, concise, audience-appropriate copy; label actions by their outcome. Explain technical terminology when necessary.
- Never hide critical warnings, required qualifiers, prices, units or irreversible consequences in a tooltip or collapsed section.

## 4. Components and interaction

Reuse components and semantic tokens. Keep state and presentation separate where practical. Every actionable component MUST actually perform its advertised action.

Design applicable default, hover, focus, pressed, selected, disabled, loading, success and error states. Disabled actions should have an understandable reason where necessary. Maintain user-entered data after recoverable failures. Provide clear feedback for pending operations; prevent accidental duplicate submissions.

Motion should communicate state, continuity or hierarchy. Prefer brief transitions (roughly 120–250 ms for small UI changes, adapted to the platform). Respect reduced-motion settings. Essential information must not depend on animation. Avoid decorative infinite loops, entrance animations that delay access, or layout jumps.

## 5. Accessibility and quality

- MUST provide accessible names, semantic roles, logical reading order, visible focus and input-method alternatives where applicable.
- For web, target WCAG 2.2 AA: normally at least 4.5:1 for ordinary text, 3:1 for qualifying large text and applicable non-text control boundaries/states. Validate actual rendered colour pairs, not just token names. Apply equivalent accessibility goals to native apps.
- Support text resizing, screen readers, high contrast where available, and reduced motion. Do not block zoom or system accessibility features.
- Design meaningful loading, empty, error, offline (where relevant) and success states; avoid fake skeleton data that could be mistaken for real values.
- Keep core tasks usable under slower connections and constrained devices. Avoid unnecessary layout shifts or heavy decorative assets.

## 6. Data visualisation (when applicable)

- Clearly distinguish measured, estimated, modelled, hypothetical and missing values. Never invent data or precision.
- Keep visual encodings and units consistent. Use a concise legend and readable annotations; do not let overlays obscure the underlying data.
- Prefer direct comparisons over charts when two numbers are sufficient.
- Provide a textual equivalent for essential visual findings and make sources and limitations discoverable.
- Treat the primary visualisation as the main content, not decoration.

## 7. Agent workflow and definition of done

1. Identify the user task and relevant project and platform rules.
2. Inspect current implementation and establish the focal point, information hierarchy and necessary states.
3. Apply shared tokens and reusable components; implement the appropriate platform layout.
4. Test representative sizes, input methods, long text, loading/error states, and accessibility.
5. Review the actual rendered interface where possible. Remove elements that add no meaningful value.
6. Report changes, tests performed, unverified states, and any justified departures from these rules.

A change is complete when the main task is clear, visual identity is consistent, interactions work, content is truthful, and the experience is readable and accessible on its supported platforms. Do not claim visual or accessibility verification that was not performed.
