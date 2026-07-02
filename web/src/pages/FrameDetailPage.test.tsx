import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...a: unknown[]) => useQueryMock(...a),
  useMutation: () => ({ mutate: vi.fn(), isPending: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));
vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
});

import { FrameDetailPage } from "./FrameDetailPage";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { ConnectError, Code } from "@connectrpc/connect";

const yamlContent = new TextEncoder().encode(
  "name: brand-voice\ndescription: voice\nversion: 1.0.0\nslots:\n  rules:\n    - no hype\n",
);

function renderDetail(permissions: { canEdit?: boolean; canDelete?: boolean } | undefined) {
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

it("shows Delete button when canDelete is true", () => {
  renderDetail({ canDelete: true });
  expect(screen.getByRole("button", { name: /^delete$/i })).toBeInTheDocument();
});

it("hides Delete button when canDelete is false", () => {
  renderDetail({ canDelete: false });
  expect(screen.queryByRole("button", { name: /^delete$/i })).not.toBeInTheDocument();
});

it("renders loading skeletons", () => {
  useQueryMock.mockReturnValue({ isLoading: true, error: null, data: undefined });
  const { container } = render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice"]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );
  expect(container.querySelector('[data-slot="skeleton"]')).toBeInTheDocument();
});

it("renders not-found on Code.NotFound", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: new ConnectError("nf", Code.NotFound), data: undefined });
  render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice"]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText(/not found/i)).toBeInTheDocument();
});

it("renders generic error on other failures", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: new ConnectError("x", Code.Internal), data: undefined });
  render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice"]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText(/could not load this frame/i)).toBeInTheDocument();
});

it("renders header, slots, and version history", () => {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrame) {
      return {
        isLoading: false, error: null,
        data: {
          frame: { name: "brand-voice", description: "voice", ownerSub: "u1", latestVersion: "1.0.0" },
          version: { version: "1.0.0", content: yamlContent, publishedBy: "pub-user", changelog: "initial release", digest: "sha256:abc123", sizeBytes: 2048n },
          extends: [], excludes: ["openteams/legacy"],
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

it("renders version metadata, changelog, and excludes", () => {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrame) {
      return {
        isLoading: false, error: null,
        data: {
          frame: { name: "brand-voice", description: "voice", ownerSub: "u1", latestVersion: "2.0.0" },
          version: { version: "1.0.0", content: yamlContent, publishedBy: "pub-user", changelog: "initial release", digest: "sha256:abc123", sizeBytes: 2048n },
          extends: [], excludes: ["openteams/legacy"],
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

  expect(screen.getByText("pub-user")).toBeInTheDocument();
  expect(screen.getByText("sha256:abc123")).toBeInTheDocument();
  expect(screen.getByText("2.0 KB")).toBeInTheDocument();
  expect(screen.getByText("initial release")).toBeInTheDocument();
  expect(screen.getByText("openteams/legacy")).toBeInTheDocument();
  // viewing v1.0.0 while latest is v2.0.0 -> shows the latest hint, not "Latest"
  expect(screen.getByText("latest: v2.0.0")).toBeInTheDocument();
});
