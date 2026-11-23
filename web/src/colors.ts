// Site colours for MapLibre paint expressions. Mirrors the :root variables
// in style.css; one meaning per colour (docs/DESIGN.md §4).
export const COLORS = {
  spare: "#4a90a4", // spare capacity
  full: "#ff9f1c", // nearly full (>= 90 %); lane drop
  cut: "#d62728", // bottleneck (min cut); confirmed jam
  idle: "#9aa5b1", // not carrying flow yet (replay start)
  path: "#1f3b57", // route being pushed right now (replay), drawn as a soft glow
  ink: "#333333",
} as const;

// Utilisation at which a road counts as nearly full.
export const NEARLY_FULL = 0.9;
