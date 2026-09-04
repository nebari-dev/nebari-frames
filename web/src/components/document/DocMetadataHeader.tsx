import { useId, type ReactNode } from "react";
import { Controller, useFormContext, useWatch } from "react-hook-form";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { FieldError, useFieldError, errorProps } from "@/components/form/FieldError";
import { VISIBILITY_VALUES } from "@/lib/frame-yaml";

// One labeled field in the metadata column: label above control, and the
// field's validation error (looked up by form path) below. The control receives
// the generated id so the Label associates with it explicitly.
function Field({
  label,
  name,
  children,
}: {
  label: string;
  name?: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children(id)}
      {name && <FieldError name={name} />}
    </div>
  );
}

// The frame's identity and spec metadata as a standard labeled form column -
// the left side of the authoring layout.
export function DocMetadataHeader({ nameReadOnly }: { nameReadOnly: boolean }) {
  const { register, control } = useFormContext();
  // Watched (not getValues) so the async edit-mode prefill re-renders the name.
  const name = useWatch({ control, name: "name" }) as string;
  const nameError = useFieldError("name");
  const descError = useFieldError("description");
  const visibilityError = useFieldError("visibility");

  return (
    <div className="space-y-4">
      {nameReadOnly ? (
        // Identity is fixed after creation; render it as a plain value.
        <div className="space-y-1.5">
          <div className="text-sm font-medium text-foreground">Name</div>
          <div className="text-lg font-semibold">{name}</div>
        </div>
      ) : (
        <Field label="Name" name="name">
          {(id) => (
            <Input
              id={id}
              {...register("name")}
              placeholder="frame-name"
              // The name is the first thing a new frame needs; land the cursor there.
              autoFocus
              {...errorProps("name", nameError)}
            />
          )}
        </Field>
      )}

      <Field label="Description" name="description">
        {(id) => (
          <Textarea
            id={id}
            rows={3}
            {...register("description")}
            placeholder="What is this frame for? One or two sentences."
            {...errorProps("description", descError)}
          />
        )}
      </Field>

      <Field label="Visibility" name="visibility">
        {(id) => (
          // Base UI's Select is not a native control, so it is driven through a
          // Controller rather than register().
          <Controller
            control={control}
            name="visibility"
            render={({ field }) => (
              <Select
                value={field.value ?? ""}
                onValueChange={(v) => field.onChange(String(v))}
              >
                <SelectTrigger
                  id={id}
                  onBlur={field.onBlur}
                  title="Declared intent that travels with the frame. Access is still governed by this org's roles and grants."
                  {...errorProps("visibility", visibilityError)}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VISIBILITY_VALUES.map((v) => (
                    <SelectItem key={v} value={v}>
                      {v}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        )}
      </Field>

      <Field label="Scope">
        {(id) => <Input id={id} {...register("scope")} placeholder="company" />}
      </Field>

      <Field label="Maintainer">
        {(id) => <Input id={id} {...register("maintainer")} placeholder="team or person" />}
      </Field>

      <Controller
        control={control}
        name="template"
        render={({ field }) => (
          <Checkbox
            checked={Boolean(field.value)}
            onCheckedChange={(checked) => field.onChange(checked)}
            description='List this Frame in the "start from a template" picker when creating new Frames.'
          >
            Offer as a template
          </Checkbox>
        )}
      />
    </div>
  );
}
