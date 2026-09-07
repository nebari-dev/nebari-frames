import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useForm, FormProvider } from "react-hook-form";
import { expect, it, vi } from "vitest";
import { FieldRuleEditor } from "./FieldRuleEditor";
import { Requirement } from "@gen/frames/v1/frame_pb";

function Harness({ onValues }: { onValues: (v: unknown) => void }) {
  const methods = useForm({ defaultValues: { rules: {} } });
  return (
    <FormProvider {...methods}>
      <FieldRuleEditor slotKey="terminology" label="Terminology" />
      <button type="button" onClick={() => onValues(methods.getValues())}>
        dump
      </button>
    </FormProvider>
  );
}

it("offers all three levels and defaults to optional", () => {
  render(<Harness onValues={vi.fn()} />);
  expect(screen.getByRole("radio", { name: /optional/i })).toBeChecked();
  expect(screen.getByRole("radio", { name: /recommended/i })).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: /required/i })).toBeInTheDocument();
});

it("records the chosen level and the note", async () => {
  const onValues = vi.fn();
  render(<Harness onValues={onValues} />);
  await userEvent.click(screen.getByRole("radio", { name: /required/i }));
  await userEvent.type(screen.getByLabelText(/note/i), "One entry per term of art.");
  await userEvent.click(screen.getByRole("button", { name: /dump/i }));
  expect(onValues).toHaveBeenCalledWith(
    expect.objectContaining({
      rules: expect.objectContaining({
        terminology: expect.objectContaining({
          level: Requirement.REQUIRED,
          note: "One entry per term of art.",
        }),
      }),
    }),
  );
});
