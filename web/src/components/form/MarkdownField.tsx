import { useFormContext } from "react-hook-form";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { FieldError, useFieldError, errorProps } from "./FieldError";
import { fillTextareaSlot } from "./fill-height";

// A markdown textarea wired to the form: the content is authored and read as
// raw markdown, so there is no rendered-preview mode.
export function MarkdownField({
  name,
  rows = 6,
  placeholder,
  ariaLabel,
  className,
  fill = false,
}: {
  name: string;
  rows?: number;
  placeholder?: string;
  ariaLabel?: string;
  className?: string;
  /**
   * Stretch the textarea to fill the height its flex parent offers, instead of
   * sizing to `rows`. The nested selectors reach the wrapper `Textarea` renders
   * around the control, which has to become the flex child for the control
   * itself to stretch.
   */
  fill?: boolean;
}) {
  const { register } = useFormContext();
  const error = useFieldError(name);
  return (
    <div
      className={cn(
        "space-y-1",
        fill &&
          cn("flex min-h-0 flex-1 flex-col space-y-0 gap-1", fillTextareaSlot),
      )}
    >
      <Textarea
        rows={fill ? undefined : rows}
        placeholder={placeholder}
        aria-label={ariaLabel}
        className={cn(fill && "flex-1", className)}
        {...register(name)}
        {...errorProps(name, error)}
      />
      <FieldError name={name} />
    </div>
  );
}
