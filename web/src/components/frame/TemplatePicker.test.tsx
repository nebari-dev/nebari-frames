import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { FrameTemplateSummarySchema } from "@gen/frames/v1/frame_pb";
import { TemplatePicker } from "./TemplatePicker";

// Built with create() rather than object literals: the generated summary is a
// protobuf-es Message and carries a $typeName a literal cannot satisfy. This is
// the same pattern DeleteFrameDialog.test.tsx and publish-errors.test.ts use.
const templates = [
  create(FrameTemplateSummarySchema, {
    id: "builtin:blank", title: "Blank", description: "Start from an empty document.", builtin: true,
  }),
  create(FrameTemplateSummarySchema, {
    id: "builtin:brand-voice", title: "Brand Voice and Style", description: "How your org sounds.", builtin: true,
  }),
  create(FrameTemplateSummarySchema, {
    id: "01JORGTEMPLATE0000000000AB", title: "Our House Style", description: "Ours.", builtin: false,
  }),
];

it("groups the org's templates separately from the built-ins", () => {
  render(<TemplatePicker templates={templates} onPick={vi.fn()} />);
  expect(screen.getByRole("heading", { name: /your organization/i })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: /built-in/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /our house style/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /blank/i })).toBeInTheDocument();
});

it("omits the organization group when the org has no templates", () => {
  render(<TemplatePicker templates={templates.filter((t) => t.builtin)} onPick={vi.fn()} />);
  expect(screen.queryByRole("heading", { name: /your organization/i })).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: /built-in/i })).toBeInTheDocument();
});

it("shows each template's description", () => {
  render(<TemplatePicker templates={templates} onPick={vi.fn()} />);
  expect(screen.getByText("How your org sounds.")).toBeInTheDocument();
});

it("reports the chosen template's id", async () => {
  const onPick = vi.fn();
  render(<TemplatePicker templates={templates} onPick={onPick} />);
  await userEvent.click(screen.getByRole("button", { name: /brand voice and style/i }));
  expect(onPick).toHaveBeenCalledWith("builtin:brand-voice");
});

it("shows the error in place of the groups, so an empty picker is never mistaken for an empty catalog", async () => {
  const onRetry = vi.fn();
  render(
    <TemplatePicker
      templates={[]}
      onPick={vi.fn()}
      failed
      onRetry={onRetry}
    />,
  );
  expect(screen.getByText(/could not be loaded/i)).toBeInTheDocument();
  // The group headings are what make a blank screen read as an answer.
  expect(screen.queryByRole("heading", { name: /built-in/i })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /try again/i }));
  expect(onRetry).toHaveBeenCalled();
});

it("shows the groups, not an error, when there is no error", () => {
  render(<TemplatePicker templates={templates} onPick={vi.fn()} failed={false} />);
  expect(screen.getByRole("heading", { name: /built-in/i })).toBeInTheDocument();
  expect(screen.queryByText(/could not be loaded/i)).not.toBeInTheDocument();
});
