import { useMemo } from "react";
import { Link, useSearchParams } from "react-router";
import { GitFork } from "lucide-react";
import { connectedTo, type HierarchyGraph, type HierarchyNode } from "@/lib/frame-hierarchy";
import { useFrameHierarchy } from "./useFrameHierarchy";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const NODE_W = 208;
const NODE_H = 68;
const COL_GAP = 40;
const ROW_GAP = 76;
const STEP_X = NODE_W + COL_GAP;
const STEP_Y = NODE_H + ROW_GAP;

interface Placed extends HierarchyNode {
  x: number;
  y: number;
}

// Center each row within the widest row so the drawing reads as a balanced tree.
function place(graph: HierarchyGraph): { placed: Map<string, Placed>; width: number; height: number } {
  const rowWidth = (n: number) => Math.max(0, n * STEP_X - COL_GAP);
  const width = Math.max(0, ...graph.rows.map((r) => rowWidth(r.length)));
  const placed = new Map<string, Placed>();
  graph.rows.forEach((row) => {
    const offset = (width - rowWidth(row.length)) / 2;
    row.forEach((node) => {
      placed.set(node.id, {
        ...node,
        x: offset + node.col * STEP_X,
        y: node.level * STEP_Y,
      });
    });
  });
  const height = Math.max(0, graph.rows.length * STEP_Y - ROW_GAP);
  return { placed, width, height };
}

function edgePath(child: Placed, parent: Placed): string {
  const cx = child.x + NODE_W / 2;
  const cy = child.y; // top of the child
  const px = parent.x + NODE_W / 2;
  const py = parent.y + NODE_H; // bottom of the parent
  const k = Math.max(24, (cy - py) * 0.5);
  return `M ${cx} ${cy} C ${cx} ${cy - k} ${px} ${py + k} ${px} ${py}`;
}

export function FrameHierarchyPage() {
  const { graph, isLoading, error } = useFrameHierarchy();
  const [params] = useSearchParams();
  const focus = params.get("focus") ?? "";

  const lineage = useMemo(
    () => (graph && focus ? connectedTo(graph, focus) : new Set<string>()),
    [graph, focus],
  );

  const layout = useMemo(() => (graph ? place(graph) : null), [graph]);

  const header = (
    <div className="space-y-1">
      <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight text-foreground">
        <GitFork className="size-5 text-muted-foreground" />
        Frame hierarchy
      </h1>
      <p className="text-sm text-muted-foreground">
        How your organization's Frames inherit from one another. Base Frames sit at the top; each
        Frame is drawn below the Frames it extends.
      </p>
    </div>
  );

  if (isLoading) {
    return (
      <div className="space-y-6 motion-safe:animate-fade-in">
        {header}
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="space-y-6">
        {header}
        <p className="text-destructive">Could not load the frame hierarchy.</p>
      </div>
    );
  }
  if (!graph || graph.nodes.length === 0) {
    return (
      <div className="space-y-6">
        {header}
        <p className="text-muted-foreground">No frames found.</p>
      </div>
    );
  }

  const { placed, width, height } = layout!;
  // Only apply focus styling when the focused frame is actually in the graph;
  // a stale ?focus= link should fall back to the normal, undimmed view.
  const hasFocus = lineage.size > 0;
  const dimmed = (id: string) => hasFocus && !lineage.has(id);

  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      {header}

      {graph.edges.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No inheritance relationships yet — no Frame extends another.
        </p>
      )}

      <div className="overflow-x-auto rounded-lg border bg-muted/20 p-6">
        <div className="relative mx-auto" style={{ width: `${width}px`, height: `${height}px` }}>
          <svg
            className="pointer-events-none absolute inset-0 overflow-visible"
            width={width}
            height={height}
            aria-hidden
          >
            {graph.edges.map((e) => {
              const c = placed.get(e.child);
              const p = placed.get(e.parent);
              if (!c || !p) return null;
              const active = hasFocus && lineage.has(e.child) && lineage.has(e.parent);
              const faded = hasFocus && !active;
              return (
                <path
                  key={`${e.child}->${e.parent}`}
                  d={edgePath(c, p)}
                  fill="none"
                  className={cn(
                    active ? "stroke-primary" : "stroke-border-strong",
                    faded && "opacity-30",
                  )}
                  strokeWidth={active ? 2 : 1.5}
                />
              );
            })}
          </svg>

          {[...placed.values()].map((node) => (
            <NodeCard
              key={node.id}
              node={node}
              focused={node.id === focus}
              dimmed={dimmed(node.id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function NodeCard({ node, focused, dimmed }: { node: Placed; focused: boolean; dimmed: boolean }) {
  const base = cn(
    "absolute flex flex-col justify-center gap-0.5 rounded-md border bg-card px-3 py-2 shadow-sm",
    "motion-safe:transition-[opacity,border-color,box-shadow]",
    dimmed && "opacity-40",
    focused && "border-primary ring-2 ring-primary/40",
  );
  const style = { left: `${node.x}px`, top: `${node.y}px`, width: `${NODE_W}px`, height: `${NODE_H}px` };

  const body = (
    <>
      <div className="flex items-center justify-between gap-2">
        <span className="truncate font-medium text-sm text-foreground">{node.name}</span>
        {node.latestVersion && (
          <Badge variant="secondary" className="shrink-0 font-mono">
            v{node.latestVersion}
          </Badge>
        )}
      </div>
      <span className="truncate text-xs text-muted-foreground">
        {node.external ? `${node.orgSlug || "external"} · not accessible` : node.orgSlug}
      </span>
    </>
  );

  if (node.external) {
    return (
      <div className={cn(base, "border-dashed")} style={style} title="Referenced Frame outside this list">
        {body}
      </div>
    );
  }
  return (
    <Link
      to={`/frames/${node.id}`}
      className={cn(base, "outline-none hover:border-foreground/40 focus-visible:ring-2 focus-visible:ring-ring")}
      style={style}
    >
      {body}
    </Link>
  );
}
