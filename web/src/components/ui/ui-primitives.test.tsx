import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { Textarea } from "./textarea";
import { Select } from "./select";
import { Dialog, DialogContent, DialogTitle } from "./dialog";

it("Textarea renders and accepts input", async () => {
  render(<Textarea aria-label="notes" />);
  await userEvent.type(screen.getByLabelText("notes"), "hi");
  expect(screen.getByLabelText("notes")).toHaveValue("hi");
});

it("Select fires onChange", async () => {
  const onChange = vi.fn();
  render(
    <Select aria-label="ver" onChange={onChange}>
      <option value="1.0.0">1.0.0</option>
      <option value="2.0.0">2.0.0</option>
    </Select>,
  );
  await userEvent.selectOptions(screen.getByLabelText("ver"), "2.0.0");
  expect(onChange).toHaveBeenCalled();
});

it("Dialog shows content only when open", () => {
  const { rerender } = render(
    <Dialog open={false} onOpenChange={() => {}}>
      <DialogContent><DialogTitle>Preview</DialogTitle></DialogContent>
    </Dialog>,
  );
  expect(screen.queryByText("Preview")).not.toBeInTheDocument();
  rerender(
    <Dialog open onOpenChange={() => {}}>
      <DialogContent><DialogTitle>Preview</DialogTitle></DialogContent>
    </Dialog>,
  );
  expect(screen.getByText("Preview")).toBeInTheDocument();
});
