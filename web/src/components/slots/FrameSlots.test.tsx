import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { FrameSlots } from "./FrameSlots";
import type { FrameDoc } from "@/lib/frame-yaml";

const doc: FrameDoc = {
  name: "x", description: "", version: "1.0.0",
  slots: {
    terminology: [{ term: "customer", definition: "an org" }],
    rules: ["no hype"],
    goals: "Be **clear**.",
  },
};

it("renders populated slots and hides empty ones", () => {
  render(<FrameSlots doc={doc} />);
  expect(screen.getByText("Terminology")).toBeInTheDocument();
  expect(screen.getByText("customer")).toBeInTheDocument();
  expect(screen.getByText("Rules")).toBeInTheDocument();
  expect(screen.getByText("Goals")).toBeInTheDocument();
  expect(screen.queryByText("Skills")).not.toBeInTheDocument();
  expect(screen.queryByText("Style")).not.toBeInTheDocument();
});
