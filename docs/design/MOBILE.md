# MOBILE.md — Phone application and mobile web UI rules

> Inherits `DESIGN.md`. Apply to iOS and Android phone apps, and to the narrow/touch layouts of responsive web apps. Native OS-specific requirements apply only to native apps or relevant hybrid integrations.

## 1. Mobile design intent

Design for handheld use, limited screen space, touch, interruptions and varying text sizes. Do not shrink desktop UI into a phone viewport. Preserve identity, content hierarchy and functionality while adapting composition and interactions.

- Make the primary task apparent; keep frequent actions within comfortable reach where practical.
- Prioritise readable content and adequate touch targets over maximum information density.
- Preserve context when revealing detail. Use a second screen for substantial tasks rather than crowding the current screen.
- Do not add tab bars, drawers or app chrome to a single-purpose product unless needed for navigation.

## 2. Layout and safe areas

- Respect safe areas, status/navigation bars, home indicators, cut-outs and system gestures. Allow decorative backgrounds to extend edge-to-edge where appropriate, but protect actionable and essential content.
- Design for narrow and short phones as well as larger ones. Use adaptive layouts and logical units, not physical device pixels.
- Reflow columns into logical reading order. Avoid nested scrolling when a single scroll surface works; maps or canvases may need their own gesture surface.
- Account for the on-screen keyboard; focused fields and primary form actions must remain reachable.
- Support portrait and relevant landscape modes; avoid orientation locks without a functional reason.
- Respect text enlargement, translated copy and long labels. Essential values, units and warnings must not be clipped.

## 3. Typography and spacing

Use the shared roles and tokens, mapped to platform units. A reasonable starting point is 16–17 for body text, 13–14 for secondary labels and 24–32 for a major heading in logical units (CSS px on web, pt on iOS, sp on Android for text). These are illustrative defaults, not fixed sizes. Support Dynamic Type / font scaling and platform accessibility preferences.

- Use concise headings; avoid oversized hero typography that pushes the main task offscreen.
- Keep vertical rhythm consistent and group with spacing instead of stacked cards.
- Start with 16 logical units of horizontal padding and adapt to the content and safe area.

## 4. Touch and gestures

- Aim for at least 44 × 44 pt targets on iOS and 48 × 48 dp on Android. On mobile web, favour roughly 44 × 44 CSS px for frequent touch controls; meet applicable web accessibility requirements. Visible icons may be smaller than their touch areas.
- Space neighbouring actions to reduce accidental activation. Keep destructive actions clearly distinct.
- Never depend on hover. Provide pressed, selected, disabled, loading and error feedback.
- Provide alternatives to complex swipes, drag-only interactions and multi-touch gestures where practical.
- Use haptics sparingly in native apps to reinforce meaningful feedback, not as the sole feedback channel.
- Respect system back and edge gestures. Avoid intercepting common gestures without a functional reason.

## 5. Navigation

Choose navigation to fit the product:

- A focused single screen for a single primary task.
- A navigation stack for hierarchical flows.
- A tab bar for a small set of frequently used top-level destinations.
- A drawer only when justified by the information architecture.

Keep the current destination clear. Preserve user state where appropriate when switching destinations. Use familiar back behaviour and do not make users rediscover their position. On mobile web, use browser history and links; do not imitate native chrome simply for appearance.

## 6. Sheets, dialogs and contextual detail

- Use a bottom sheet for brief contextual details, filters or actions that benefit from keeping the underlying content visible.
- Use a full screen for long forms or complex editing; use a dialog for a short, important decision.
- Provide a visible dismissal action where needed; do not require a swipe to close. Avoid stacked sheets and preserve focus or selection on dismissal.
- Adapt sheet height to the content and available space; if the content grows, allow expansion or scrolling. Do not obscure the main feature permanently.
- For maps and canvases, move or fit the selected feature into the unobscured region when a sheet opens. Provide accessible zoom and selection alternatives.

## 7. Forms and device behaviour

- Minimise typing; use suitable keyboard/input types, autofill and native pickers where appropriate.
- Keep labels visible and validation actionable. Preserve progress across recoverable errors and interruptions.
- Account for slow connections, offline conditions where relevant and app backgrounding. Restore state when returning to an interrupted task where practical.
- Use purposeful transitions and respect reduced-motion settings. Do not block access while purely decorative animation completes.

## 8. Platform conventions

**iOS:** Respect safe areas, system presentation and navigation conventions; support VoiceOver and Dynamic Type. Use native controls where familiarity or accessibility benefits. Keep swipe-back and dismissal predictable.

**Android:** Respect system insets, edge-to-edge layout, back navigation and gesture areas; support TalkBack and scalable text. Use Material conventions where they improve usability. Handle system back consistently for sheets, dialogs and navigation.

**Mobile web:** Respect browser history, viewport resizing, dynamic browser toolbars and pinch zoom. Use responsive CSS and actual pointer capabilities rather than user-agent assumptions. Avoid inaccessible faux native UI and keyboard traps. Do not require installation for basic browser functionality unless the project explicitly requires it.

The platforms need not be pixel-identical. Preserve identity and semantics, not implementation details.

## 9. Mobile acceptance checks

Test representative logical phone widths (e.g. 320, 375, 390 and 430 on web or comparable native devices), short viewports, increased text size, keyboard open, and landscape when supported.

- The primary task remains visible and usable; no desktop layout is merely squeezed down.
- Touch targets and spacing are comfortable; nothing essential requires hover or a hidden gesture.
- Safe areas, browser/OS bars and keyboard do not obscure essential actions.
- Navigation, dismissal and back behaviour are predictable.
- Text scales and wraps without clipping critical information.
- Screen-reader labels and reading order are meaningful.
- Loading, empty, error, offline where relevant, and interrupted states are handled.
- Contextual sheets preserve useful underlying content; maps and visualisations remain legible.
- Report actual test coverage and unverified states.
