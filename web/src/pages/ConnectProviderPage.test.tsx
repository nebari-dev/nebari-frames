import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it } from "vitest";
import { ConnectProviderPage } from "./ConnectProviderPage";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/connect/:provider" element={<ConnectProviderPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

it("renders steps and the runtime connector URL for an available provider", () => {
  renderAt("/connect/claude");
  expect(screen.getByRole("heading", { name: /claude\.ai/i })).toBeInTheDocument();
  // connector URL is origin + /mcp (jsdom origin is http://localhost)
  expect(screen.getByText(`${window.location.origin}/mcp`)).toBeInTheDocument();
  // first step title rendered
  expect(screen.getByText(/open connector settings/i)).toBeInTheDocument();
  // last-verified footer
  expect(screen.getByText(/last verified/i)).toBeInTheDocument();
});

it("renders a not-available state for an unknown provider", () => {
  renderAt("/connect/nope");
  expect(screen.getByText(/not available yet/i)).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /back to connect/i })).toHaveAttribute(
    "href",
    "/connect",
  );
});

it("renders steps and the connector URL for the now-available ChatGPT provider", () => {
  renderAt("/connect/chatgpt");
  expect(screen.getByRole("heading", { name: /chatgpt/i })).toBeInTheDocument();
  expect(screen.getByText(`${window.location.origin}/mcp`)).toBeInTheDocument();
  expect(screen.getByText(/enable developer mode/i)).toBeInTheDocument();
});
