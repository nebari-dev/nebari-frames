import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: (...a: unknown[]) => useQueryMock(...a) }));

import { FrameDetailPage } from "./FrameDetailPage";
import { FrameService } from "@gen/frames/v1/frame_service_pb";

const yamlContent = new TextEncoder().encode(
  "name: brand-voice\ndescription: voice\nversion: 1.0.0\nslots:\n  rules:\n    - no hype\n",
);

function renderDetail(permissions: { canEdit: boolean } | undefined) {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrame) {
      return {
        isLoading: false, error: null,
        data: {
          frame: { name: "brand-voice", description: "voice", ownerSub: "u1" },
          version: { version: "1.0.0", content: yamlContent },
          extends: [], permissions,
        },
      };
    }
    return { isLoading: false, error: null, data: { versions: [] } };
  });
  render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice"]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );
}

it("shows Edit link when canEdit is true", () => {
  renderDetail({ canEdit: true });
  expect(screen.getByRole("link", { name: /edit/i })).toHaveAttribute("href", "/frames/openteams/brand-voice/edit");
});

it("hides Edit link when canEdit is false", () => {
  renderDetail({ canEdit: false });
  expect(screen.queryByRole("link", { name: /edit/i })).not.toBeInTheDocument();
});

it("renders header, slots, and version history", () => {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrame) {
      return {
        isLoading: false, error: null,
        data: {
          frame: { name: "brand-voice", description: "voice", ownerSub: "u1" },
          version: { version: "1.0.0", content: yamlContent },
          extends: [],
        },
      };
    }
    return { isLoading: false, error: null, data: { versions: [{ version: "1.0.0", changelog: "init", publishedBy: "u1" }] } };
  });

  render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice"]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );

  expect(screen.getByRole("heading", { name: "brand-voice" })).toBeInTheDocument();
  expect(screen.getByText("Rules")).toBeInTheDocument();
  expect(screen.getByText("no hype")).toBeInTheDocument();
  expect(screen.getByText(/Version history/)).toBeInTheDocument();
});
