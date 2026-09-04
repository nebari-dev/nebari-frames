import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

const resolved = `name: brand-voice
description: d
version: 1.0.0
body: |
  Merged rule.
`;
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: { resolvedContent: new TextEncoder().encode(resolved) }, isLoading: false, error: null }),
}));

import { ResolvedPreview } from "./ResolvedPreview";

it("renders the resolved body in the dialog when open", () => {
  render(<ResolvedPreview org="openteams" name="brand-voice" version="1.0.0" open onClose={() => {}} />);
  expect(screen.getByText("Merged rule.")).toBeInTheDocument();
});

it("renders nothing when closed", () => {
  render(<ResolvedPreview org="openteams" name="brand-voice" version="1.0.0" open={false} onClose={() => {}} />);
  expect(screen.queryByText("Merged rule.")).not.toBeInTheDocument();
});
