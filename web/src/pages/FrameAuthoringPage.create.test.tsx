import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

const navigateMock = vi.fn();
vi.mock("react-router", async (orig) => ({
  ...(await orig<typeof import("react-router")>()),
  useNavigate: () => navigateMock,
  useBlocker: () => ({ state: "unblocked" }),
}));

const mutateMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: { org: { slug: "openteams" } }, isLoading: false, error: null }),
  useMutation: () => ({ mutate: mutateMock, isPending: false, isSuccess: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { FrameAuthoringPage } from "./FrameAuthoringPage";

it("publishes a filled form and calls the mutation", async () => {
  render(<MemoryRouter><FrameAuthoringPage mode="create" /></MemoryRouter>);
  await userEvent.type(screen.getByPlaceholderText("brand-voice"), "brand-voice");
  await userEvent.type(screen.getByPlaceholderText("OpenTeams brand voice"), "desc");
  await userEvent.type(screen.getByPlaceholderText("1.0.0"), "1.0.0");
  const publishButtons = screen.getAllByRole("button", { name: /^publish$/i });
  await userEvent.click(publishButtons[publishButtons.length - 1]);
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());
  const arg = mutateMock.mock.calls[0][0];
  expect(new TextDecoder().decode(arg.content)).toMatch(/name: brand-voice/);
});
