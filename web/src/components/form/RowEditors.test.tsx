import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useForm, FormProvider } from "react-hook-form";
import { expect, it } from "vitest";
import type { ReactNode } from "react";
import { TerminologyEditor } from "./TerminologyEditor";
import { ListEditor } from "./ListEditor";

function Wrap({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { slots: { terminology: [], rules: [] } } });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

it("TerminologyEditor adds and removes rows", async () => {
  render(<Wrap><TerminologyEditor /></Wrap>);
  await userEvent.click(screen.getByRole("button", { name: /add term/i }));
  expect(screen.getByPlaceholderText("Term")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /remove/i }));
  expect(screen.queryByPlaceholderText("Term")).not.toBeInTheDocument();
});

it("ListEditor adds a row", async () => {
  render(<Wrap><ListEditor name="slots.rules" label="Rules" /></Wrap>);
  await userEvent.click(screen.getByRole("button", { name: /add rule/i }));
  expect(screen.getByRole("textbox")).toBeInTheDocument();
});
