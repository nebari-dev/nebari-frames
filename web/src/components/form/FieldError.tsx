import { useFormContext, get } from "react-hook-form";

// Reads the message at a react-hook-form error path. Paths are dotted and may
// index arrays ("slots.terminology.2.definition"), which is what get() handles.
//
// The `root` fallback is not an alternative spelling: @hookform/resolvers
// (toNestErrors) nests an error under `<path>.root` whenever that path has
// registered children, which is what a rule about a whole array produces - a
// duplicate-term refine over `slots.terminology`, say, while
// `slots.terminology.0.term` is registered. Reading only `.message` there
// finds an object with none, so the form refuses to submit and shows nothing:
// a Save button that silently does nothing at all.
export function useFieldError(name: string): string | undefined {
  const { formState } = useFormContext();
  const e = get(formState.errors, name) as
    | { message?: string; root?: { message?: string } }
    | undefined;
  const message = typeof e?.message === "string" ? e.message : e?.root?.message;
  return typeof message === "string" ? message : undefined;
}

// Stable id for the error text, so an input can point at it via
// aria-describedby without each caller inventing its own convention.
export function errorId(name: string): string {
  return `err-${name.replace(/[^a-zA-Z0-9]+/g, "-")}`;
}

// Props to spread onto the input a message belongs to, wiring up the
// accessibility relationship between the two.
export function errorProps(name: string, message: string | undefined) {
  return message
    ? { "aria-invalid": true as const, "aria-describedby": errorId(name) }
    : {};
}

// Renders the validation message for one field path, if there is one. Server
// violations set via setError land here too, which is what makes a failed
// publish visible on the input that caused it.
export function FieldError({ name }: { name: string }) {
  const message = useFieldError(name);
  if (!message) return null;
  return (
    <span id={errorId(name)} role="alert" className="block text-xs text-destructive">
      {message}
    </span>
  );
}
