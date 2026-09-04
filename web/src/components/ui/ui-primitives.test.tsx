import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { Textarea } from "./textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./select";
import { Dialog, DialogContent, DialogTitle } from "./dialog";

it("Textarea renders and accepts input", async () => {
  render(<Textarea aria-label="notes" />);
  await userEvent.type(screen.getByLabelText("notes"), "hi");
  expect(screen.getByLabelText("notes")).toHaveValue("hi");
});

it("Select opens its listbox and fires onValueChange", async () => {
  const onValueChange = vi.fn();
  render(
    <Select onValueChange={onValueChange}>
      <SelectTrigger aria-label="ver">
        <SelectValue placeholder="version..." />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="1.0.0">1.0.0</SelectItem>
        <SelectItem value="2.0.0">2.0.0</SelectItem>
      </SelectContent>
    </Select>,
  );
  await userEvent.click(screen.getByRole("combobox", { name: "ver" }));
  await userEvent.click(await screen.findByRole("option", { name: "2.0.0" }));
  expect(onValueChange).toHaveBeenCalledWith("2.0.0", expect.anything());
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
