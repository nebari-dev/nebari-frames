import { useState } from "react";
import { Link, useSearchParams } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { GitFork, LayoutGrid, Plus, Search, Table2 } from "lucide-react";
import { filterFrames } from "@/lib/filter";
import { FramesTable } from "@/components/frame/FramesTable";
import { FrameHierarchyView } from "@/components/frame/FrameHierarchyView";
import { useFrameHierarchy } from "./useFrameHierarchy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const CARD_GRID = "grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(20rem,1fr))]";

const VIEWS = ["cards", "table", "hierarchy"] as const;
type View = (typeof VIEWS)[number];

const VIEW_META: { value: View; label: string; icon: typeof LayoutGrid }[] = [
  { value: "cards", label: "Cards", icon: LayoutGrid },
  { value: "table", label: "Table", icon: Table2 },
  { value: "hierarchy", label: "Hierarchy", icon: GitFork },
];

export function CatalogPage() {
  const [query, setQuery] = useState("");
  const [params, setParams] = useSearchParams();
  const view: View = VIEWS.includes(params.get("view") as View) ? (params.get("view") as View) : "cards";
  const focus = params.get("focus") ?? "";

  const { data, isLoading, error } = useQuery(FrameService.method.listFrames, {});

  function setView(next: View) {
    setParams(
      (prev) => {
        const p = new URLSearchParams(prev);
        if (next === "cards") p.delete("view");
        else p.set("view", next);
        if (next !== "hierarchy") p.delete("focus");
        return p;
      },
      { replace: true },
    );
  }

  const frames = filterFrames(data?.frames ?? [], query);
  const showSearch = view !== "hierarchy";

  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Frames</h1>
        <p className="text-sm text-muted-foreground">
          Browse, author, and connect your organization's Frames.
        </p>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4">
        {showSearch && (
          <div className="relative w-full max-w-[41rem]">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              placeholder="Search frames…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="h-9 pl-9"
            />
          </div>
        )}
        <div className="ml-auto flex items-center gap-3">
          <ViewToggle view={view} onChange={setView} />
          {data?.canCreate && (
            <Button render={<Link to="/frames/new" />}>
              <Plus />
              Create new Frame
            </Button>
          )}
        </div>
      </div>

      {view === "hierarchy" ? (
        <HierarchyView focus={focus} />
      ) : isLoading ? (
        <div className={CARD_GRID}>
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-28" />)}
        </div>
      ) : error ? (
        <p className="text-destructive">Could not load frames.</p>
      ) : frames.length === 0 ? (
        <p className="text-muted-foreground">No frames found.</p>
      ) : view === "table" ? (
        <FramesTable frames={frames} />
      ) : (
        <div className={CARD_GRID}>
          {frames.map((f) => (
            <Link key={`${f.orgSlug}/${f.name}`} to={`/frames/${f.orgSlug}/${f.name}`}>
              <Card className="p-4 h-full hover:border-foreground/30 transition-colors">
                <div className="font-medium">{f.name}</div>
                <div className="text-sm text-muted-foreground line-clamp-2">{f.description}</div>
                <div className="text-xs text-muted-foreground mt-3">
                  {f.ownerSub} - v{f.latestVersion}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function ViewToggle({ view, onChange }: { view: View; onChange: (v: View) => void }) {
  return (
    <div className="inline-flex gap-1 rounded-md border border-border/50 bg-muted/40 p-1" role="group" aria-label="Frame view">
      {VIEW_META.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          aria-label={label}
          title={label}
          aria-pressed={view === value}
          onClick={() => onChange(value)}
          className={cn(
            "flex items-center justify-center rounded-[6px] p-1.5 outline-none",
            "motion-safe:transition-colors focus-visible:ring-2 focus-visible:ring-ring",
            view === value
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          <Icon className="size-4" />
        </button>
      ))}
    </div>
  );
}

// Fetches the org-wide inheritance graph. Isolated in its own component so the
// (multi-request) hierarchy fetch only runs when this view is actually shown.
function HierarchyView({ focus }: { focus: string }) {
  const { graph, isLoading, error } = useFrameHierarchy();

  if (isLoading) return <Skeleton className="h-96 w-full" />;
  if (error) return <p className="text-destructive">Could not load the frame hierarchy.</p>;
  if (!graph || graph.nodes.length === 0) return <p className="text-muted-foreground">No frames found.</p>;

  return (
    <div className="space-y-3">
      {graph.edges.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No inheritance relationships yet — no Frame extends another.
        </p>
      )}
      <FrameHierarchyView graph={graph} focus={focus} />
    </div>
  );
}
