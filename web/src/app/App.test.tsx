import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

// Mock auth infrastructure so AuthProvider resolves without real OIDC
vi.mock("@/lib/auth/authConfig", () => ({
  fetchAuthConfig: vi.fn(async () => ({ issuer: "https://idp", clientId: "web" })),
}));
vi.mock("@/lib/auth/userManager", () => ({
  buildUserManager: () => ({
    getUser: vi.fn(async () => ({ expired: false, access_token: "tok", profile: { email: "test@example.com" } })),
    signinRedirect: vi.fn(),
    removeUser: vi.fn(),
  }),
}));

// Mock connect-query so RequireMembership does not fire a real gRPC call
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: vi.fn(() => ({ isLoading: false, error: null, data: { canCreate: false } })),
  TransportProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// Mock transport to avoid real fetch
vi.mock("@/lib/transport", () => ({
  createTransport: vi.fn(() => ({})),
}));

import { Providers } from "./Providers";
import { App } from "./App";

it("renders the app title", async () => {
  render(
    <MemoryRouter initialEntries={["/"]}>
      <Providers>
        <App />
      </Providers>
    </MemoryRouter>,
  );
  await waitFor(() => expect(screen.getByRole("img", { name: "Nebari" })).toBeInTheDocument());
  expect(screen.getByRole("link", { name: /nebari frames home/i })).toBeInTheDocument();
});
