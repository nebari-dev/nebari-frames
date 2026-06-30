import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";
import { ConnectHubPage } from "./ConnectHubPage";

it("links all three available providers (Claude, ChatGPT, Gemini)", () => {
  render(
    <MemoryRouter>
      <ConnectHubPage />
    </MemoryRouter>,
  );
  expect(screen.getByRole("link", { name: /claude\.ai/i })).toHaveAttribute(
    "href",
    "/connect/claude",
  );
  expect(screen.getByRole("link", { name: /chatgpt/i })).toHaveAttribute(
    "href",
    "/connect/chatgpt",
  );
  expect(screen.getByRole("link", { name: /gemini/i })).toHaveAttribute(
    "href",
    "/connect/gemini",
  );
});
