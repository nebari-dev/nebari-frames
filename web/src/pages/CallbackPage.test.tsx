import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

const completeLogin = vi.fn(async () => {});
const navigate = vi.fn();
vi.mock("@/lib/auth/useAuth", () => ({ useAuth: () => ({ completeLogin }) }));
vi.mock("react-router", async (orig) => ({
  ...(await orig<typeof import("react-router")>()),
  useNavigate: () => navigate,
}));

import { ThemeProvider } from "@/lib/theme/ThemeContext";
import { CallbackPage } from "./CallbackPage";

function renderCallback() {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <CallbackPage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

it("completes login then redirects home", async () => {
  renderCallback();
  await waitFor(() => expect(completeLogin).toHaveBeenCalledOnce());
  await waitFor(() => expect(navigate).toHaveBeenCalledWith("/", { replace: true }));
});

it("shows an error if the exchange fails", async () => {
  completeLogin.mockRejectedValueOnce(new Error("bad code"));
  renderCallback();
  await waitFor(() => expect(screen.getByText(/sign-in failed/i)).toBeInTheDocument());
});
