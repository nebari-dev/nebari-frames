import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";
import type { FrameSummary } from "@gen/frames/v1/frame_pb";

vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: undefined, isLoading: false, error: null }),
  useMutation: () => ({ mutate: vi.fn(), isPending: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { FramesTable } from "./FramesTable";

const frames = [
  {
    orgSlug: "openteams", name: "brand-voice", description: "d", ownerSub: "u1",
    latestVersion: "1.0.0", permissions: { canEdit: true, canDelete: true },
  },
  {
    orgSlug: "openteams", name: "read-only", description: "d", ownerSub: "u2",
    latestVersion: "1.0.0", permissions: { canEdit: false, canDelete: false },
  },
] as unknown as FrameSummary[];

it("shows icon actions only for permitted frames", () => {
  render(<MemoryRouter><FramesTable frames={frames} /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /edit brand-voice/i })).toHaveAttribute(
    "href",
    "/frames/openteams/brand-voice/edit",
  );
  expect(screen.getByRole("button", { name: /delete brand-voice/i })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /edit read-only/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /delete read-only/i })).not.toBeInTheDocument();
});

it("opens the delete confirmation from the row action", async () => {
  render(<MemoryRouter><FramesTable frames={frames} /></MemoryRouter>);
  await userEvent.click(screen.getByRole("button", { name: /delete brand-voice/i }));
  expect(await screen.findByText(/delete brand-voice\?/i)).toBeInTheDocument();
});

const many = Array.from({ length: 23 }, (_, i) => ({
  orgSlug: "openteams",
  name: `frame-${String(i + 1).padStart(2, "0")}`,
  description: "d",
  ownerSub: "u1",
  latestVersion: "1.0.0",
  permissions: { canEdit: false, canDelete: false },
})) as unknown as FrameSummary[];

it("shows ten rows per page and pages through the rest", async () => {
  render(<MemoryRouter><FramesTable frames={many} /></MemoryRouter>);

  // Header row plus ten body rows.
  expect(screen.getAllByRole("row")).toHaveLength(11);
  expect(screen.getByRole("link", { name: "frame-01" })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "frame-11" })).not.toBeInTheDocument();
  expect(screen.getByText(/showing 1–10 of 23 frames/i)).toBeInTheDocument();

  await userEvent.click(screen.getByRole("button", { name: /next/i }));
  expect(screen.getByRole("link", { name: "frame-11" })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "frame-01" })).not.toBeInTheDocument();
  expect(screen.getByText(/showing 11–20 of 23 frames/i)).toBeInTheDocument();

  // Last page is partial, and Next stops there.
  await userEvent.click(screen.getByRole("button", { name: /next/i }));
  expect(screen.getAllByRole("row")).toHaveLength(4);
  expect(screen.getByRole("button", { name: /next/i })).toBeDisabled();

  await userEvent.click(screen.getByRole("button", { name: /previous/i }));
  expect(screen.getByText(/showing 11–20 of 23 frames/i)).toBeInTheDocument();
});

it("still shows the row count on a single page, with paging disabled", () => {
  render(<MemoryRouter><FramesTable frames={frames} /></MemoryRouter>);
  expect(screen.getByText(/showing 1–2 of 2 frames/i)).toBeInTheDocument();
  expect(screen.getByText(/page 1 of 1/i)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /previous/i })).toBeDisabled();
  expect(screen.getByRole("button", { name: /next/i })).toBeDisabled();
});
