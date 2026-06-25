import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi, beforeEach } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: (...a: unknown[]) => useQueryMock(...a) }));

import { FramePicker } from "./FramePicker";
import { FrameService } from "@gen/frames/v1/frame_service_pb";

beforeEach(() => useQueryMock.mockReset());

it("filters frames by typed text and emits ref on select", async () => {
  // Key on method identity (the existing test convention in FrameDetailPage.test.tsx).
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.listFrameVersions) {
      return { data: { versions: [{ version: "1.0.0" }, { version: "2.0.0" }] } };
    }
    return {
      data: { frames: [
        { orgSlug: "openteams", name: "company", description: "" },
        { orgSlug: "openteams", name: "hipaa", description: "" },
      ] },
    };
  });
  const onChange = vi.fn();
  render(<FramePicker value={{ ref: "", version: "" }} onChange={onChange} withVersion />);

  await userEvent.type(screen.getByPlaceholderText(/search frames/i), "comp");
  await userEvent.click(screen.getByText("openteams/company"));
  expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ ref: "openteams/company" }));
});
