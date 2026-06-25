import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useForm, FormProvider } from "react-hook-form";
import { expect, it } from "vitest";
import type { ReactNode } from "react";
import { MarkdownField } from "./MarkdownField";

function Wrap({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { slots: { goals: "# Hello" } } });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

it("toggles between textarea and rendered preview", async () => {
  render(<Wrap><MarkdownField name="slots.goals" label="Goals" /></Wrap>);
  expect(screen.getByRole("textbox")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /preview/i }));
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Hello" })).toBeInTheDocument();
});
