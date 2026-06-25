import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";
import { ConnectHubPage } from "./ConnectHubPage";

it("links the available Claude provider and disables coming-soon ones", () => {
  render(
    <MemoryRouter>
      <ConnectHubPage />
    </MemoryRouter>,
  );
  const claudeLink = screen.getByRole("link", { name: /claude\.ai/i });
  expect(claudeLink).toHaveAttribute("href", "/connect/claude");

  // coming-soon providers are present but not links
  expect(screen.getByText(/chatgpt/i)).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /chatgpt/i })).not.toBeInTheDocument();
  expect(screen.getAllByText(/coming soon/i).length).toBeGreaterThan(0);
});
