// Package flow implements max flow (Dinic's algorithm) and min cut.
// It is pure: no I/O and no imports from other internal packages.
package flow

// Inf is used for super-source / super-sink arcs.
const Inf = int(^uint(0) >> 2)

type arc struct {
	to, rev, cap, orig int
}

// Network is a directed flow network on vertices 0..n-1.
type Network struct {
	adj   [][]arc
	level []int
	iter  []int
	ids   [][2]int // arc id -> (vertex, index in adj)
}

// New creates a network with n vertices.
func New(n int) *Network {
	return &Network{adj: make([][]arc, n)}
}

// AddArc adds a directed arc u->v with capacity c and returns its arc id.
func (g *Network) AddArc(u, v, c int) int {
	g.adj[u] = append(g.adj[u], arc{to: v, rev: len(g.adj[v]), cap: c, orig: c})
	g.adj[v] = append(g.adj[v], arc{to: u, rev: len(g.adj[u]) - 1, cap: 0, orig: 0})
	g.ids = append(g.ids, [2]int{u, len(g.adj[u]) - 1})
	return len(g.ids) - 1
}

// Flow returns the flow currently on arc id.
func (g *Network) Flow(id int) int {
	p := g.ids[id]
	a := g.adj[p[0]][p[1]]
	return a.orig - a.cap
}

func (g *Network) bfs(s, t int) bool {
	g.level = make([]int, len(g.adj))
	for i := range g.level {
		g.level[i] = -1
	}
	g.level[s] = 0
	q := []int{s}
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		for _, a := range g.adj[u] {
			if a.cap > 0 && g.level[a.to] < 0 {
				g.level[a.to] = g.level[u] + 1
				q = append(q, a.to)
			}
		}
	}
	return g.level[t] >= 0
}

func (g *Network) dfs(u, t, f int) int {
	if u == t {
		return f
	}
	for ; g.iter[u] < len(g.adj[u]); g.iter[u]++ {
		a := &g.adj[u][g.iter[u]]
		if a.cap > 0 && g.level[u] < g.level[a.to] {
			if d := g.dfs(a.to, t, min(f, a.cap)); d > 0 {
				a.cap -= d
				g.adj[a.to][a.rev].cap += d
				return d
			}
		}
	}
	return 0
}

// MaxFlow computes the max flow from s to t. Call once per Network.
func (g *Network) MaxFlow(s, t int) int {
	total := 0
	for g.bfs(s, t) {
		g.iter = make([]int, len(g.adj))
		for f := g.dfs(s, t, Inf); f > 0; f = g.dfs(s, t, Inf) {
			total += f
		}
	}
	return total
}

// MinCut returns the ids of arcs crossing the source-side min cut.
// Must be called after MaxFlow. Min cuts are not unique; this is the
// cut nearest the source.
func (g *Network) MinCut(s int) []int {
	seen := g.fromSource(s)
	var cut []int
	for id, p := range g.ids {
		a := g.adj[p[0]][p[1]]
		if seen[p[0]] && !seen[a.to] && a.orig > 0 {
			cut = append(cut, id)
		}
	}
	return cut
}

// InEveryMinCut returns the ids of arcs that lie in every minimum s-t cut,
// in arc id order. Must be called after MaxFlow.
//
// Picard & Queyranne (1980) show that the minimum cuts correspond to the
// closed sets of the residual graph. Two of them are the extremes: the cut
// nearest the source, whose source side S holds the vertices reachable
// from s, and the cut nearest the sink, whose sink side T holds the
// vertices that can reach t. An arc u->v is in every min cut exactly when
// u is in S and v is in T. These are the only arcs where adding any
// capacity raises the max flow. Every other arc has an equally small cut
// that avoids it, so widening that arc alone gains nothing.
func (g *Network) InEveryMinCut(s, t int) []int {
	src := g.fromSource(s)
	snk := g.toSink(t)
	var ids []int
	for id, p := range g.ids {
		a := g.adj[p[0]][p[1]]
		if src[p[0]] && snk[a.to] && a.orig > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// fromSource marks vertices reachable from s in the residual graph.
func (g *Network) fromSource(s int) []bool {
	seen := make([]bool, len(g.adj))
	seen[s] = true
	q := []int{s}
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		for _, a := range g.adj[u] {
			if a.cap > 0 && !seen[a.to] {
				seen[a.to] = true
				q = append(q, a.to)
			}
		}
	}
	return seen
}

// toSink marks vertices that can reach t in the residual graph. It walks
// arcs backwards: u reaches v in the residual when the arc u->v has spare
// capacity, and that arc's reverse is stored at v as a.rev in adj[a.to].
func (g *Network) toSink(t int) []bool {
	seen := make([]bool, len(g.adj))
	seen[t] = true
	q := []int{t}
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		for _, b := range g.adj[v] {
			// b is v->u; its partner u->v has residual capacity
			// g.adj[u][b.rev].cap.
			u := b.to
			if !seen[u] && g.adj[u][b.rev].cap > 0 {
				seen[u] = true
				q = append(q, u)
			}
		}
	}
	return seen
}
