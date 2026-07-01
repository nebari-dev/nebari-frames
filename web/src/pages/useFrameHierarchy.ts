import { useQuery } from "@tanstack/react-query";
import { useTransport } from "@connectrpc/connect-query";
import { createClient } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import {
  buildHierarchy,
  parentRefId,
  type EdgeInput,
  type FrameNodeInput,
  type HierarchyGraph,
} from "@/lib/frame-hierarchy";

// Fetches every frame the caller can read, then each frame's `extends` edges
// (the list endpoint omits them), and assembles the org-wide inheritance graph.
export function useFrameHierarchy(): {
  graph: HierarchyGraph | undefined;
  isLoading: boolean;
  error: unknown;
} {
  const transport = useTransport();
  const { data, isLoading, error } = useQuery({
    queryKey: ["frame-hierarchy"],
    queryFn: async () => {
      const client = createClient(FrameService, transport);
      const { frames } = await client.listFrames({});
      const nodes: FrameNodeInput[] = frames.map((f) => ({
        id: parentRefId(f.orgSlug, f.name),
        orgSlug: f.orgSlug,
        name: f.name,
        description: f.description,
        latestVersion: f.latestVersion,
      }));

      const edges: EdgeInput[] = [];
      await Promise.all(
        frames.map(async (f) => {
          const child = parentRefId(f.orgSlug, f.name);
          try {
            const resp = await client.getFrame({ orgSlug: f.orgSlug, name: f.name });
            for (const p of resp.extends) {
              edges.push({ child, parent: p.ref, version: p.version });
            }
          } catch {
            // A frame that fails to load contributes no edges; it still appears
            // as an isolated node from the list response.
          }
        }),
      );

      return buildHierarchy(nodes, edges);
    },
  });

  return { graph: data, isLoading, error };
}
