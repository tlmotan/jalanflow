// Splits the max flow on each road into whole routes from the start gate
// to the end gate, so a vehicle can be drawn driving one route end to end
// instead of looping on a single road. Pure: tested with `npm test`.
//
// Nothing new is computed about traffic here: the routes add up to exactly
// the flow_vph the Go pipeline exported for every road.

export interface FlowEdge {
  from: string; // node key (the road's first coordinate)
  to: string; // node key (the road's last coordinate)
  flow: number; // veh/h, > 0 to take part
}

export interface FlowRoute {
  edges: number[]; // indexes into the input, in travel order
  flow: number; // veh/h carried along the whole route
}

/**
 * Greedy flow decomposition. Each pass starts at the node with the most
 * flow still leaving it unaccounted for, follows the fullest road out of
 * each node until none is left (the end gate), and removes that much flow.
 * Flow going round in a circle (none in practice) comes out as a route
 * that stops where it would repeat a node.
 */
export function decompose(edges: FlowEdge[]): FlowRoute[] {
  const left = edges.map((e) => Math.max(0, e.flow));
  const out = new Map<string, number[]>();
  const excess = new Map<string, number>(); // flow out minus flow in
  edges.forEach((e, i) => {
    if (left[i]! <= 0) return;
    out.set(e.from, [...(out.get(e.from) ?? []), i]);
    excess.set(e.from, (excess.get(e.from) ?? 0) + left[i]!);
    excess.set(e.to, (excess.get(e.to) ?? 0) - left[i]!);
  });

  const routes: FlowRoute[] = [];
  // Every pass empties a road or a start node, so this ends.
  for (let guard = 0; guard < edges.length * 2 + 2; guard++) {
    let start: string | undefined;
    let most = 0;
    for (const [node, x] of excess) {
      if (x > most) {
        most = x;
        start = node;
      }
    }
    if (start === undefined) {
      const i = left.findIndex((f) => f > 0);
      if (i < 0) break;
      start = edges[i]!.from;
      most = Infinity;
    }

    const path: number[] = [];
    const seen = new Set([start]);
    let node = start;
    let amount = most;
    for (;;) {
      let best = -1;
      for (const i of out.get(node) ?? []) {
        if (left[i]! > 0 && (best < 0 || left[i]! > left[best]!)) best = i;
      }
      if (best < 0) break;
      path.push(best);
      amount = Math.min(amount, left[best]!);
      node = edges[best]!.to;
      if (seen.has(node)) break;
      seen.add(node);
    }
    if (path.length === 0) {
      excess.delete(start);
      continue;
    }
    for (const i of path) left[i]! -= amount;
    excess.set(start, (excess.get(start) ?? 0) - amount);
    excess.set(node, (excess.get(node) ?? 0) + amount);
    routes.push({ edges: path, flow: amount });
  }
  return routes;
}
