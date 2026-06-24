import { render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("@/lib/auth/authConfig", () => ({
  fetchAuthConfig: vi.fn(async () => ({ issuer: "https://idp", clientId: "web" })),
}));
vi.mock("@/lib/auth/userManager", () => ({
  buildUserManager: () => ({ getUser: vi.fn(async () => null) }),
}));

import { Providers } from "./Providers";

it("renders children inside the provider tree", async () => {
  render(<Providers><div>child-ok</div></Providers>);
  await waitFor(() => expect(screen.getByText("child-ok")).toBeInTheDocument());
});
