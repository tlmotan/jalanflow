// Mirrors Go output structs (internal/export, internal/fixes). Change both together.

export type Direction = "inbound" | "outbound";
export type Difficulty = "easy" | "medium" | "hard";

export interface EdgeProps {
  id: number;
  dir: Direction;
  name: string;
  ref: string;
  class: string;
  lanes: number;
  lanes_inferred: boolean;
  capacity_vph: number;
  flow_vph: number;
  utilisation: number;
  is_cut: boolean;
}

export interface Fix {
  edge_id: number;
  direction: Direction;
  change: string;
  gain_vph: number;
  new_max_flow_vph: number;
  next_cut_edges: number[];
  difficulty: Difficulty;
  reasons: string[];
}

export type FixesFile = Record<Direction, Fix[]>;

export interface Manifest {
  corridor: string;
  generated_at: string;
  osm_date: string;
  pipeline_version: string;
  capacity_config_hash: string;
  max_flow_vph: Record<Direction, number>;
  assumptions: string[];
}
