import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AuthProvider } from "./AuthProvider";
import { useAuth } from "./useAuth";

import { fetchAuthConfig } from "./authConfig";

vi.mock("./authConfig", () => ({
  fetchAuthConfig: vi.fn(async () => ({ enabled: true, issuer: "https://idp", clientId: "web" })),
}));

const getUser = vi.fn();
vi.mock("./userManager", () => ({
  buildUserManager: () => ({ getUser, signinRedirect: vi.fn(), removeUser: vi.fn() }),
}));

afterEach(() => vi.clearAllMocks());

function Probe() {
  const { status } = useAuth();
  return <div>status:{status}</div>;
}

it("starts anonymous when no stored user", async () => {
  getUser.mockResolvedValue(null);
  render(<AuthProvider><Probe /></AuthProvider>);
  await waitFor(() => expect(screen.getByText("status:anonymous")).toBeInTheDocument());
});

it("is authenticated when a valid user is stored", async () => {
  getUser.mockResolvedValue({ expired: false, access_token: "tok" });
  render(<AuthProvider><Probe /></AuthProvider>);
  await waitFor(() => expect(screen.getByText("status:authenticated")).toBeInTheDocument());
});

it("is authenticated without OIDC when auth is disabled (dev mode)", async () => {
  vi.mocked(fetchAuthConfig).mockResolvedValueOnce({ enabled: false, issuer: "", clientId: "" });
  render(<AuthProvider><Probe /></AuthProvider>);
  await waitFor(() => expect(screen.getByText("status:authenticated")).toBeInTheDocument());
  // No UserManager is built, so getUser is never consulted.
  expect(getUser).not.toHaveBeenCalled();
});
