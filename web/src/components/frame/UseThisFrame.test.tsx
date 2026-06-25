import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";
import { UseThisFrame } from "./UseThisFrame";

it("shows the MCP URI and a link to the Connect hub", () => {
  render(
    <MemoryRouter>
      <UseThisFrame org="openteams" name="brand-voice" />
    </MemoryRouter>,
  );
  // MCP URI rendered (origin is http://localhost in jsdom)
  expect(screen.getByText(/\/mcp#openteams\/brand-voice$/)).toBeInTheDocument();
  // link into Connect pages
  const link = screen.getByRole("link", { name: /set up a connector/i });
  expect(link).toHaveAttribute("href", "/connect");
});
