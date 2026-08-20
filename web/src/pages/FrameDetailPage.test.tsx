import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
const publishMutateMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...a: unknown[]) => useQueryMock(...a),
  useMutation: () => ({ mutate: publishMutateMock, isPending: false }),
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
  "name: brand-voice\ndescription: voice\nversion: 1.0.0\nbody: |\n  ## Rules\n\n  no hype\n",
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

// Mock a frame whose latest is 2.0.0 while the page is viewing 1.0.0
// (initialEntries carries ?v=1.0.0 when viewingOld).
function mockVersioned({ canEdit = false, viewingOld = false } = {}) {
  useQueryMock.mockImplementation((method: unknown, input?: { version?: string }) => {
    if (method === FrameService.method.getFrame) {
      const v = viewingOld && input?.version === "1.0.0" ? "1.0.0" : "2.0.0";
      return {
        isLoading: false, error: null,
        data: {
          frame: { name: "brand-voice", description: "voice", ownerSub: "u1", latestVersion: "2.0.0" },
          version: {
            version: v, content: yamlContent, publishedBy: "pub-user",
            changelog: "initial release", digest: "sha256:abc123", sizeBytes: 2048n,
          },
          extends: [], excludes: ["openteams/legacy"], permissions: { canEdit },
        },
      };
    }
    return {
      isLoading: false, error: null,
      data: {
        versions: [
          { version: "2.0.0", changelog: "second", publishedBy: "u1" },
          { version: "1.0.0", changelog: "init", publishedBy: "u1" },
        ],
      },
    };
  });
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes><Route path="/frames/:org/:name" element={<FrameDetailPage />} /></Routes>
    </MemoryRouter>,
  );
}

it("renders the header with a read-only edit form and no tabs", () => {
  mockVersioned();
  renderAt("/frames/openteams/brand-voice");

  expect(screen.getByRole("heading", { name: "brand-voice" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /back to frames/i })).toHaveAttribute("href", "/");
  expect(screen.queryByRole("tab")).not.toBeInTheDocument();

  // The view mirrors the edit form, read-only: same fields, none editable.
  const nameInput = screen.getByLabelText(/^name$/i);
  expect(nameInput).toHaveValue("brand-voice");
  expect(nameInput).toHaveAttribute("readonly");
  expect(screen.getByDisplayValue("openteams/legacy")).toHaveAttribute("readonly");

  // The markdown source sits in a read-only Content textarea, verbatim.
  const content = screen.getByLabelText(/^content$/i);
  expect(content).toHaveAttribute("readonly");
  expect(content).toHaveValue("## Rules\n\nno hype\n");
});

it("opens version history from the header button, with links to view past versions", async () => {
  mockVersioned();
  renderAt("/frames/openteams/brand-voice");

  await userEvent.click(screen.getByRole("button", { name: /versions \(2\)/i }));

  // Every published version is listed; past versions link to a pinned view.
  expect(await screen.findByText("init")).toBeInTheDocument();
  expect(screen.getByText("second")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /view v1\.0\.0/i })).toHaveAttribute(
    "href",
    "/frames/openteams/brand-voice?v=1.0.0",
  );
  // The version on screen (latest) has no view link.
  expect(screen.queryByRole("link", { name: /view v2\.0\.0/i })).not.toBeInTheDocument();
});

it("offers restore only when viewing a past version with edit rights", () => {
  mockVersioned({ canEdit: true, viewingOld: true });
  renderAt("/frames/openteams/brand-voice?v=1.0.0");

  expect(screen.getByText("latest: v2.0.0")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /view latest/i })).toHaveAttribute(
    "href",
    "/frames/openteams/brand-voice",
  );
  expect(screen.getByRole("button", { name: /restore this version/i })).toBeInTheDocument();
});

it("hides restore on the latest version and without edit rights", () => {
  mockVersioned({ canEdit: true });
  renderAt("/frames/openteams/brand-voice");
  expect(screen.queryByRole("button", { name: /restore this version/i })).not.toBeInTheDocument();

  mockVersioned({ canEdit: false, viewingOld: true });
  renderAt("/frames/openteams/brand-voice?v=1.0.0");
  expect(screen.queryByRole("button", { name: /restore this version/i })).not.toBeInTheDocument();
});

it("republishes the viewed version's content after confirming a restore", async () => {
  mockVersioned({ canEdit: true, viewingOld: true });
  renderAt("/frames/openteams/brand-voice?v=1.0.0");

  await userEvent.click(screen.getByRole("button", { name: /restore this version/i }));
  expect(await screen.findByText(/restore v1\.0\.0\?/i)).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /^restore$/i }));

  expect(publishMutateMock).toHaveBeenCalledTimes(1);
  const [req] = publishMutateMock.mock.calls[0] as [{ content: Uint8Array; changelog: string }];
  expect(req.changelog).toBe("Restore of v1.0.0");
  const yaml = new TextDecoder().decode(req.content);
  // The restored content carries the old body under a bumped version number.
  expect(yaml).toContain("version: 2.0.1");
  expect(yaml).toContain("no hype");
});
