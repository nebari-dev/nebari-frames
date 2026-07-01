import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";

const authMock = vi.fn();
vi.mock("@/lib/auth/useAuth", () => ({ useAuth: () => authMock() }));

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));

import { ThemeProvider } from "@/lib/theme/ThemeContext";
import { Header } from "./Header";

// Base UI menus open/select on a real pointer sequence that userEvent.click
// doesn't reproduce under jsdom; drive them with explicit pointer events.
function pointerClick(el: Element) {
  fireEvent.pointerDown(el);
  fireEvent.pointerUp(el);
  fireEvent.click(el);
}

function renderHeader() {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <Header />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

afterEach(() => {
  cleanup();
  document.documentElement.classList.remove("dark");
});

it("shows the name and a Log out action when authenticated", async () => {
  authMock.mockReturnValue({
    status: "authenticated",
    user: { profile: { email: "u@example.com" } },
    login: vi.fn(),
    logout: vi.fn(),
  });
  useQueryMock.mockReturnValue({ data: { email: "u@example.com", role: "viewer" } });
  renderHeader();

  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  expect(await screen.findByRole("menuitem", { name: /log out/i })).toBeInTheDocument();
});

it("offers a Log in action when anonymous", async () => {
  authMock.mockReturnValue({ status: "anonymous", user: null, login: vi.fn(), logout: vi.fn() });
  useQueryMock.mockReturnValue({ data: undefined });
  renderHeader();

  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  expect(await screen.findByRole("menuitem", { name: /log in/i })).toBeInTheDocument();
});

it("toggles dark mode from the account menu theme switcher", async () => {
  authMock.mockReturnValue({ status: "anonymous", user: null, login: vi.fn(), logout: vi.fn() });
  useQueryMock.mockReturnValue({ data: undefined });
  renderHeader();

  expect(document.documentElement.classList.contains("dark")).toBe(false);
  pointerClick(screen.getByRole("button", { name: /account menu/i }));
  pointerClick(await screen.findByRole("menuitemradio", { name: /dark/i }));
  expect(document.documentElement.classList.contains("dark")).toBe(true);
});
