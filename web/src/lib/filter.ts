import type { FrameSummary } from "@gen/frames/v1/frame_pb";

// Case-insensitive substring match over name + description. Empty query
// returns the full list. Server-side search is a roadmap item.
export function filterFrames(frames: FrameSummary[], query: string): FrameSummary[] {
  const q = query.trim().toLowerCase();
  if (!q) return frames;
  return frames.filter(
    (f) => f.name.toLowerCase().includes(q) || f.description.toLowerCase().includes(q),
  );
}
