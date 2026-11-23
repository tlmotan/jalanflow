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
  /** This +1 lane alone raises max flow (edge is in every min cut). */
  helps: boolean;
  /** Plain-language reason for zero gain; absent when helps. */
  why_no_gain?: string;
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

// Mirrors internal/lanedrop (Drop, Road, Confirmation) and export.LaneDropsFile.
export type LaneDropKind = "merge" | "narrowing";

export interface LatLon {
  lat: number;
  lon: number;
}

export interface LaneDropRoad {
  osm_way_id: number;
  name: string;
  ref: string;
  class: string;
  lanes: number;
  lanes_inferred: boolean;
  leaves: string; // unnamed ramps: nearest named road where it starts ("" if none)
  joins: string; // unnamed ramps: nearest named road where it ends ("" if none)
}

export interface LaneDropConfirmation {
  name: string;
  source: string;
}

export interface LaneDrop {
  rank: number;
  key: string;
  kind: LaneDropKind;
  pos: LatLon;
  lanes_in: number;
  lanes_out: number;
  lanes_lost: number;
  squeeze: number;
  from: LaneDropRoad[];
  to: LaneDropRoad[];
  lanes_inferred: boolean;
  confirmed: LaneDropConfirmation | null;
}

export interface LaneDropsFile {
  corridor: string;
  osm_date: string;
  generated_at: string;
  ranking: string;
  assumptions: string[];
  drops: LaneDrop[];
}
