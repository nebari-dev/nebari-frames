import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));
vi.mock("@/components/frame/DeleteFrameDialog", () => ({
  DeleteFrameDialog: ({ org, name }: { org: string; name: string }) => <div>dialog {org}/{name}</div>,
}));

import { AdminFramesPage } from "./AdminFramesPage";

it("lists all org frames", () => {
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: null,
    data: { frames: [{ orgSlug: "openteams", name: "brand-voice", description: "d", latestVersion: "1.0.0" }] },
  });
  render(<MemoryRouter><AdminFramesPage /></MemoryRouter>);
  expect(screen.getByText("brand-voice")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /delete/i })).toBeInTheDocument();
});
