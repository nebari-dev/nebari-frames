import { useFormContext } from "react-hook-form";
import { Textarea } from "@/components/ui/textarea";

export function ChangelogField() {
  const { register } = useFormContext();
  return (
    <label className="block space-y-1">
      <span className="text-sm font-medium">Changelog (this version)</span>
      <Textarea rows={3} {...register("changelog")} placeholder="What changed in this version?" />
    </label>
  );
}
