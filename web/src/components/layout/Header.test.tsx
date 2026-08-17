import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";

const authMock = vi.fn();
vi.mock("@/lib/auth/useAuth", () => ({ useAuth: () => authMock() }));

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));

import { THEME_STORAGE_KEY } from "@/app/Providers";
import { ThemeProvider } from "@/hooks/theme-provider";
import { setBranding } from "@/lib/branding";
import { Header } from "./Header";

// Base UI menus open/select on a real pointer sequence that userEvent.click
// doesn't reproduce under jsdom; drive them with explicit pointer events.
function pointerClick(el: Element) {
  fireEvent.pointerDown(el);
  fireEvent.pointerUp(el);
  fireEvent.click(el);
}

function renderHeader(path = "/") {
  return render(
    <ThemeProvider storageKey={THEME_STORAGE_KEY}>
      <MemoryRouter initialEntries={[path]}>
        <Header />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

function mockAuthenticated() {
  authMock.mockReturnValue({
    status: "authenticated",
    user: { profile: { email: "u@example.com" } },
    login: vi.fn(),
    logout: vi.fn(),
  });
  useQueryMock.mockReturnValue({ data: { email: "u@example.com", role: "viewer" } });
}

afterEach(() => {
  cleanup();
  setBranding(null);
  localStorage.removeItem(THEME_STORAGE_KEY);
  document.documentElement.classList.remove("dark");
});

it("shows the name and a Sign out action when authenticated", async () => {
  mockAuthenticated();
  renderHeader();

  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  expect(await screen.findByRole("menuitem", { name: /sign out/i })).toBeInTheDocument();
});

it("signs out from the account menu", async () => {
  const logout = vi.fn();
  authMock.mockReturnValue({
    status: "authenticated",
    user: { profile: { email: "u@example.com" } },
    login: vi.fn(),
    logout,
  });
  useQueryMock.mockReturnValue({ data: { email: "u@example.com", role: "viewer" } });
  renderHeader();

  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  pointerClick(await screen.findByRole("menuitem", { name: /sign out/i }));
  expect(logout).toHaveBeenCalledOnce();
});

it("offers a Sign in button when anonymous", () => {
  authMock.mockReturnValue({ status: "anonymous", user: null, login: vi.fn(), logout: vi.fn() });
  useQueryMock.mockReturnValue({ data: undefined });
  renderHeader();

  expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /account menu/i })).not.toBeInTheDocument();
});

it("marks the nav link for the current section as the current page", () => {
  mockAuthenticated();
  renderHeader("/connect/github");

  expect(screen.getByRole("link", { name: "Connect" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("link", { name: "Frames" })).not.toHaveAttribute("aria-current");
});

it("links the logo to the homepage", () => {
  mockAuthenticated();
  renderHeader();

  expect(screen.getByRole("link", { name: /go to homepage/i })).toHaveAttribute("href", "/");
});

it("shows the bundled Nebari wordmark when the deployment is unbranded", () => {
  mockAuthenticated();
  renderHeader();

  const logo = screen.getByRole("img", { name: "Nebari" });
  expect(logo).toHaveAttribute("src", expect.stringContaining("nebari-logo_light"));
});

it("shows the branded logo and title when branding is configured", () => {
  setBranding({ title: "Acme Frames", logoUrl: "https://cdn.acme.example/logo.svg" });
  mockAuthenticated();
  renderHeader();

  expect(screen.getByRole("img", { name: "Acme Frames" })).toHaveAttribute(
    "src",
    "https://cdn.acme.example/logo.svg",
  );
});

it("exposes the theme picker as menuitemradio items with aria-checked", async () => {
  mockAuthenticated();
  renderHeader();

  pointerClick(screen.getByRole("button", { name: /account menu/i }));

  expect(await screen.findByRole("menuitemradio", { name: /system theme/i })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  expect(screen.getByRole("menuitemradio", { name: /light mode/i })).toHaveAttribute(
    "aria-checked",
    "false",
  );
  expect(screen.getByRole("menuitemradio", { name: /dark mode/i })).toHaveAttribute(
    "aria-checked",
    "false",
  );
});

it("toggles dark mode from the account menu theme switcher", async () => {
  mockAuthenticated();
  renderHeader();

  expect(document.documentElement.classList.contains("dark")).toBe(false);
  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  pointerClick(await screen.findByRole("menuitemradio", { name: /dark mode/i }));

  expect(document.documentElement.classList.contains("dark")).toBe(true);
  expect(await screen.findByRole("menuitemradio", { name: /dark mode/i })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  // The preference persists under the app's pre-registry storage key.
  expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
});
