import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { CopyField } from "./CopyField";

it("renders the value and label", () => {
  render(<CopyField label="Connector URL" value="https://hub.example/mcp" />);
  expect(screen.getByText("Connector URL")).toBeInTheDocument();
  expect(screen.getByText("https://hub.example/mcp")).toBeInTheDocument();
});

it("copies the value to the clipboard and shows confirmation", async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.assign(navigator, { clipboard: { writeText } });
  render(<CopyField value="https://hub.example/mcp" copyLabel="Copy URL" />);
  await userEvent.click(screen.getByRole("button", { name: /copy url/i }));
  expect(writeText).toHaveBeenCalledWith("https://hub.example/mcp");
  expect(screen.getByRole("button", { name: /copied/i })).toBeInTheDocument();
});
