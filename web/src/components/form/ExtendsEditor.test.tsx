import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useForm, FormProvider } from "react-hook-form";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";

vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: { frames: [{ orgSlug: "openteams", name: "company", description: "" }], versions: [] } }),
}));

import { ExtendsEditor } from "./ExtendsEditor";

function Wrap({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { extends: [] } });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

it("adds and removes a parent row", async () => {
  render(<Wrap><ExtendsEditor /></Wrap>);
  await userEvent.click(screen.getByRole("button", { name: /add parent/i }));
  expect(screen.getByPlaceholderText(/search frames/i)).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /remove parent/i }));
  expect(screen.queryByPlaceholderText(/search frames/i)).not.toBeInTheDocument();
});
