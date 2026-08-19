import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router";
import { expect, it, vi } from "vitest";

vi.mock("react-router", async (orig) => ({
  ...(await orig<typeof import("react-router")>()),
  useNavigate: () => vi.fn(),
}));

// Delegate to a mock-prefixed fn (vitest's hoisting-safe naming exception) so we
// can configure per-method returns inside the test body where FrameService is in scope.
const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...a: unknown[]) => useQueryMock(...a),
  useMutation: () => ({ mutate: vi.fn(), isPending: false, isSuccess: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { FrameAuthoringPage } from "./FrameAuthoringPage";
import { FrameService } from "@gen/frames/v1/frame_service_pb";

const sample = `name: brand-voice
description: OpenTeams brand voice
version: 1.2.0
slots:
  rules:
    - Cite benchmarks.
`;

it("pre-fills the document with the name fixed and suggests a bumped version", async () => {
  useQueryMock.mockImplementation((method: unknown) =>
    method === FrameService.method.getFrame
      ? { data: { frame: { name: "brand-voice" }, version: { version: "1.2.0", digest: "d1", content: new TextEncoder().encode(sample) } }, isLoading: false }
      : { data: { org: { slug: "openteams" } }, isLoading: false },
  );
  render(
    <MemoryRouter initialEntries={["/frames/openteams/brand-voice/edit"]}>
      <Routes><Route path="/frames/:org/:name/edit" element={<FrameAuthoringPage mode="edit" />} /></Routes>
    </MemoryRouter>,
  );
  // Identity is fixed after creation: the name renders as the title, not an input.
  await waitFor(() => expect(screen.getByText("brand-voice")).toBeInTheDocument());
  expect(screen.queryByLabelText(/frame name/i)).not.toBeInTheDocument();

  // Sections carrying content are on the page without any adding.
  expect(screen.getByDisplayValue("Cite benchmarks.")).toBeInTheDocument();

  // The bumped version suggestion lives in the publish dialog.
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  expect(await screen.findByDisplayValue("1.2.1")).toBeInTheDocument();
});
