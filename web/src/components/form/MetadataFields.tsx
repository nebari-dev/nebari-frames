import { useFormContext } from "react-hook-form";
import { Input } from "@/components/ui/input";

function fieldError(errors: Record<string, unknown>, key: string): string | undefined {
  const e = errors[key] as { message?: string } | undefined;
  return e?.message;
}

export function MetadataFields({ nameReadOnly }: { nameReadOnly: boolean }) {
  const { register, formState: { errors } } = useFormContext();
  return (
    <div className="space-y-3">
      <label className="block space-y-1">
        <span className="text-sm font-medium">Name</span>
        <Input {...register("name")} readOnly={nameReadOnly} placeholder="brand-voice" />
        {fieldError(errors as Record<string, unknown>, "name") && (
          <span className="text-xs text-destructive">{fieldError(errors as Record<string, unknown>, "name")}</span>
        )}
      </label>
      <label className="block space-y-1">
        <span className="text-sm font-medium">Description</span>
        <Input {...register("description")} placeholder="OpenTeams brand voice" />
        {fieldError(errors as Record<string, unknown>, "description") && (
          <span className="text-xs text-destructive">{fieldError(errors as Record<string, unknown>, "description")}</span>
        )}
      </label>
      <label className="block space-y-1">
        <span className="text-sm font-medium">Version</span>
        <Input {...register("version")} placeholder="1.0.0" />
        {fieldError(errors as Record<string, unknown>, "version") && (
          <span className="text-xs text-destructive">{fieldError(errors as Record<string, unknown>, "version")}</span>
        )}
      </label>
    </div>
  );
}
