import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { MarkdownView } from "./MarkdownView";

it("renders markdown headings and emphasis", () => {
  render(<MarkdownView source={"# Title\n\nsome **bold** text"} />);
  expect(screen.getByRole("heading", { name: "Title" })).toBeInTheDocument();
  expect(screen.getByText("bold")).toBeInTheDocument();
});

it("does not render raw script tags", () => {
  // react-markdown drops raw HTML blocks (including script) without rehype-raw.
  // "safe" is on a separate paragraph so it is not part of the HTML block.
  render(<MarkdownView source={"<script>window.x=1</script>\n\nsafe"} />);
  expect(document.querySelector("script")).toBeNull();
  expect(screen.getByText(/safe/)).toBeInTheDocument();
});
