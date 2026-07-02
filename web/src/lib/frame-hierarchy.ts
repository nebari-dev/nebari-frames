// Builds an org-wide inheritance graph from frames and their `extends` edges,
// then assigns each node a layered position (row = depth, col = order within
// the row) suitable for a top-down DAG drawing.
//
// Orientation: an edge points from a child (the frame that declares `extends`)
// to its parent (the frame being extended). Base frames that extend nothing sit
// at the top (level 0); every frame is drawn strictly below all of its parents.

export interface FrameNodeInput {
  id: string; // "orgSlug/name"
  orgSlug: string;
  name: string;
  description: string;
  latestVersion: string;
}

export interface EdgeInput {
  child: string; // node id that declares the `extends`
  parent: string; // referenced parent id ("orgSlug/name")
  version: string; // pinned parent version
}

export interface HierarchyNode extends FrameNodeInput {
  level: number; // row: longest ancestor chain length
  col: number; // order within the row
  external: boolean; // parent ref we have no frame record for (e.g. another org)
}

export interface HierarchyEdge {
  child: string;
  parent: string;
  version: string;
}

export interface HierarchyGraph {
  nodes: HierarchyNode[];
  edges: HierarchyEdge[];
  rows: HierarchyNode[][]; // nodes grouped by level, each row ordered by col
}

export function parentRefId(orgSlug: string, name: string): string {
  return `${orgSlug}/${name}`;
}

// Longest-path leveling with cycle protection. A frame's level is one more than
// the deepest level among its parents; a parent we cannot resolve (or a back
// edge from an accidental cycle) contributes nothing.
function computeLevels(
  ids: string[],
  parentsOf: Map<string, string[]>,
): Map<string, number> {
  const level = new Map<string, number>();
  const inProgress = new Set<string>();

  function visit(id: string): number {
    const cached = level.get(id);
    if (cached !== undefined) return cached;
    if (inProgress.has(id)) return 0; // cycle guard: ignore the back edge
    inProgress.add(id);
    let max = -1;
    for (const parent of parentsOf.get(id) ?? []) {
      max = Math.max(max, visit(parent));
    }
    inProgress.delete(id);
    const lvl = max + 1;
    level.set(id, lvl);
    return lvl;
  }

  for (const id of ids) visit(id);
  return level;
}

export function buildHierarchy(
  frames: FrameNodeInput[],
  edges: EdgeInput[],
): HierarchyGraph {
  const nodeMap = new Map<string, HierarchyNode>();
  for (const f of frames) {
    nodeMap.set(f.id, { ...f, level: 0, col: 0, external: false });
  }

  // Keep only edges whose child we know about; synthesize placeholder nodes for
  // any parent ref that has no frame record so its edges still render.
  const keptEdges: HierarchyEdge[] = [];
  const parentsOf = new Map<string, string[]>();
  for (const e of edges) {
    if (!nodeMap.has(e.child)) continue;
    if (!nodeMap.has(e.parent)) {
      const [orgSlug = "", name = e.parent] = e.parent.split("/");
      nodeMap.set(e.parent, {
        id: e.parent,
        orgSlug,
        name,
        description: "",
        latestVersion: "",
        level: 0,
        col: 0,
        external: true,
      });
    }
    keptEdges.push({ child: e.child, parent: e.parent, version: e.version });
    const list = parentsOf.get(e.child) ?? [];
    list.push(e.parent);
    parentsOf.set(e.child, list);
  }

  const ids = [...nodeMap.keys()];
  const levels = computeLevels(ids, parentsOf);
  for (const node of nodeMap.values()) {
    node.level = levels.get(node.id) ?? 0;
  }

  // Group by level, then order each row. Level 0 is ordered by id; deeper rows
  // use the barycenter (mean parent column) to reduce edge crossings, falling
  // back to id for stability and ties.
  const maxLevel = Math.max(0, ...[...levels.values()]);
  const rows: HierarchyNode[][] = [];
  const colOf = new Map<string, number>();
  for (let lvl = 0; lvl <= maxLevel; lvl++) {
    const row = [...nodeMap.values()].filter((n) => n.level === lvl);
    row.sort((a, b) => {
      const ba = barycenter(a.id, parentsOf, colOf);
      const bb = barycenter(b.id, parentsOf, colOf);
      if (ba !== bb) return ba - bb;
      return a.id.localeCompare(b.id);
    });
    row.forEach((node, i) => {
      node.col = i;
      colOf.set(node.id, i);
    });
    rows.push(row);
  }

  return { nodes: [...nodeMap.values()], edges: keptEdges, rows };
}

function barycenter(
  id: string,
  parentsOf: Map<string, string[]>,
  colOf: Map<string, number>,
): number {
  const parents = parentsOf.get(id) ?? [];
  const cols = parents.map((p) => colOf.get(p)).filter((c): c is number => c !== undefined);
  if (cols.length === 0) return Number.POSITIVE_INFINITY; // no positioned parents -> sort last, then by id
  return cols.reduce((a, b) => a + b, 0) / cols.length;
}

// The set of node ids connected to `focusId` by following edges in both
// directions (all ancestors and all descendants). Used to highlight the lineage
// of a single frame within the org-wide graph. Returns an empty set if the id is
// absent.
export function connectedTo(graph: HierarchyGraph, focusId: string): Set<string> {
  const known = new Set(graph.nodes.map((n) => n.id));
  if (!known.has(focusId)) return new Set();

  const childrenOf = new Map<string, string[]>();
  const parentsOf = new Map<string, string[]>();
  for (const e of graph.edges) {
    (parentsOf.get(e.child) ?? parentsOf.set(e.child, []).get(e.child)!).push(e.parent);
    (childrenOf.get(e.parent) ?? childrenOf.set(e.parent, []).get(e.parent)!).push(e.child);
  }

  const result = new Set<string>([focusId]);
  const walk = (start: string, adj: Map<string, string[]>) => {
    const stack = [start];
    while (stack.length > 0) {
      const cur = stack.pop()!;
      for (const next of adj.get(cur) ?? []) {
        if (!result.has(next)) {
          result.add(next);
          stack.push(next);
        }
      }
    }
  };
  walk(focusId, parentsOf);
  walk(focusId, childrenOf);
  return result;
}
