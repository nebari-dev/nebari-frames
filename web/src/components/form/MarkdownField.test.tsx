import { render, screen } from "@testing-library/react";
import { useForm, FormProvider } from "react-hook-form";
import { expect, it } from "vitest";
import type { ReactNode } from "react";
import { MarkdownField } from "./MarkdownField";

function Wrap({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { body: "# Hello" } });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

it("renders the registered value as raw markdown in a textarea", () => {
  render(<Wrap><MarkdownField name="body" ariaLabel="Content" /></Wrap>);
  expect(screen.getByLabelText(/^content$/i)).toHaveValue("# Hello");
  // Raw source only - no rendered-preview toggle.
  expect(screen.queryByRole("button", { name: /preview/i })).not.toBeInTheDocument();
});
