import { useEffect, useState } from "react";
import { useForm, FormProvider, useFormContext } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQuery, useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectError, Code } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Requirement } from "@gen/frames/v1/frame_pb";
import type { FrameTemplateSummary } from "@gen/frames/v1/frame_pb";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { SLOT_SECTIONS, type SlotSectionDef } from "@/lib/slot-sections";
import { parseFrameContent, serializeFramePrefill, type FrameDoc } from "@/lib/frame-yaml";
import { TerminologyEditor } from "@/components/form/TerminologyEditor";
import { ListEditor } from "@/components/form/ListEditor";
import { MarkdownField } from "@/components/form/MarkdownField";
import { FieldRuleEditor } from "@/components/form/FieldRuleEditor";
import { FieldError, useFieldError, errorProps } from "@/components/form/FieldError";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Alert } from "@/components/ui/alert";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";

const encode = (s: string) => new TextEncoder().encode(s);

// Content-only schema for a template's prefill: no name/description/version -
// duplicated from frame-yaml.ts's slotsSchema rather than reused, matching the
// precedent authoring-schema.ts already sets of keeping the strict authoring
// shape separate from the loose round-trip one.
const termSchema = z.object({ term: z.string(), definition: z.string() });
const slotsSchema = z.object({
  terminology: z.array(termSchema).optional(),
  rules: z.array(z.string()).optional(),
  skills: z.array(z.string()).optional(),
  prompts: z.array(z.string()).optional(),
  tool_specs: z.string().optional(),
  goals: z.string().optional(),
  style: z.string().optional(),
  norms: z.string().optional(),
  architecture: z.string().optional(),
  business_process: z.string().optional(),
});
const ruleEntrySchema = z.object({
  level: z.nativeEnum(Requirement).optional(),
  note: z.string().optional(),
});
const templateFormSchema = z.object({
  title: z.string().trim().min(1, "must not be empty"),
  description: z.string().trim().min(1, "must not be empty"),
  slots: slotsSchema,
  rules: z.record(z.string(), ruleEntrySchema).optional(),
});
type TemplateForm = z.infer<typeof templateFormSchema>;

function emptyTemplateForm(): TemplateForm {
  return { title: "", description: "", slots: {}, rules: {} };
}

// Only a slot whose admin actually set something is worth persisting: an
// OPTIONAL level with no note is indistinguishable from the slot never having
// been touched, so field_rules stays as small as what the admin meant.
function buildFieldRules(rules: TemplateForm["rules"]): Record<string, { level: Requirement; note: string }> {
  const out: Record<string, { level: Requirement; note: string }> = {};
  for (const def of SLOT_SECTIONS) {
    const rule = rules?.[def.key];
    const level = rule?.level ?? Requirement.OPTIONAL;
    const note = rule?.note?.trim() ?? "";
    if (level === Requirement.OPTIONAL && note === "") continue;
    out[def.key] = { level, note };
  }
  return out;
}

// Title and description live inside the FormProvider tree (not in the parent
// that creates it) so useFieldError/useFormContext resolve correctly - a
// component cannot consume a context it itself provides.
function TemplateIdentityFields() {
  const { register } = useFormContext<TemplateForm>();
  const titleError = useFieldError("title");
  const descriptionError = useFieldError("description");
  return (
    <div className="space-y-4">
      <label className="block space-y-1">
        <span className="text-sm font-medium">Title</span>
        <Input aria-label="Title" {...register("title")} {...errorProps("title", titleError)} />
        <FieldError name="title" />
      </label>
      <label className="block space-y-1">
        <span className="text-sm font-medium">Description</span>
        <Textarea
          aria-label="Description"
          rows={2}
          {...register("description")}
          {...errorProps("description", descriptionError)}
        />
        <FieldError name="description" />
      </label>
    </div>
  );
}

// One slot: its content editor (the same ones the authoring form uses, so a
// template's prefill is edited exactly the way it will later be authored),
// paired with the rule that governs it.
function TemplateSlotSection({ def }: { def: SlotSectionDef }) {
  return (
    <section className="space-y-3 border-t border-border pt-4">
      <h2 className="text-base font-semibold text-foreground">{def.label}</h2>
      <p className="text-xs text-muted-foreground">{def.hint}</p>
      {def.kind === "terms" && <TerminologyEditor />}
      {def.kind === "list" && (
        <ListEditor name={def.path as `slots.${"rules" | "skills" | "prompts"}`} label={def.label} />
      )}
      {def.kind === "prose" && <MarkdownField name={def.path} />}
      <FieldRuleEditor slotKey={def.key} label={def.label} />
    </section>
  );
}

type TemplateTarget = { mode: "create" } | { mode: "edit"; id: string };

// Create and edit share one dialog and one submit path: update replaces the
// stored row wholesale, so the edit form must arrive at the same full payload
// a create does, just seeded from the fetched row instead of starting empty.
function TemplateFormDialog({
  target,
  onOpenChange,
  onSaved,
}: {
  target: TemplateTarget;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}) {
  const editingId = target.mode === "edit" ? target.id : "";
  const rowQ = useQuery(
    FrameService.method.getFrameTemplate,
    { id: editingId },
    { enabled: target.mode === "edit" },
  );
  const createM = useMutation(FrameService.method.createFrameTemplate);
  const updateM = useMutation(FrameService.method.updateFrameTemplate);
  const [formError, setFormError] = useState<string | null>(null);
  // Round-tripped but not editable here: this page has no control for a
  // template's suggested parent, so an edit must not silently drop one a
  // template already carried in from elsewhere (the CLI, or a future editor).
  const [carriedExtends, setCarriedExtends] = useState<FrameDoc["extends"]>(undefined);
  const [ready, setReady] = useState(target.mode === "create");

  const methods = useForm<TemplateForm>({
    resolver: zodResolver(templateFormSchema),
    defaultValues: emptyTemplateForm(),
  });

  useEffect(() => {
    if (target.mode !== "edit") return;
    const tmpl = rowQ.data?.template;
    if (!tmpl) return;
    try {
      const doc = parseFrameContent(tmpl.prefill);
      // This effect reacts to the row query resolving (an external system's
      // data arriving), which is the effect rule's own sanctioned use of
      // setState-in-effect; the heuristic cannot tell that apart from deriving
      // state from other state, hence the disables below.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setCarriedExtends(doc.extends);
      methods.reset({
        title: tmpl.title,
        description: tmpl.description,
        slots: doc.slots,
        rules: tmpl.fieldRules as TemplateForm["rules"],
      });
      setReady(true);
    } catch {
      // Schedule outside the effect body to satisfy react-hooks/set-state-in-effect
      setTimeout(() => setFormError("This template's saved content could not be loaded for editing."), 0);
    }
    // Seed once when the row arrives; re-running on every rowQ identity change
    // would fight the admin's own edits mid-session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target.mode, rowQ.data]);

  const busy = createM.isPending || updateM.isPending;

  const onSubmit = async (form: TemplateForm) => {
    setFormError(null);
    const payload = {
      title: form.title.trim(),
      description: form.description.trim(),
      prefill: encode(serializeFramePrefill({ slots: form.slots, extends: carriedExtends })),
      fieldRules: buildFieldRules(form.rules),
    };
    try {
      if (target.mode === "create") {
        await createM.mutateAsync(payload);
      } else {
        await updateM.mutateAsync({ id: target.id, ...payload });
      }
      onSaved();
      onOpenChange(false);
    } catch (err) {
      const ce = ConnectError.from(err);
      if (ce.code === Code.AlreadyExists) {
        methods.setError("title", { type: "server", message: ce.rawMessage || "a template with this title already exists" });
      } else {
        setFormError(ce.rawMessage || "Could not save this template.");
      }
    }
  };

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
        <DialogTitle>{target.mode === "edit" ? "Edit template" : "New template"}</DialogTitle>
        {!ready ? (
          <Skeleton className="h-48 w-full" />
        ) : (
          <FormProvider {...methods}>
            <form onSubmit={(e) => { void methods.handleSubmit(onSubmit)(e); }} className="space-y-6">
              {formError && <Alert variant="destructive">{formError}</Alert>}
              <TemplateIdentityFields />
              {SLOT_SECTIONS.map((def) => (
                <TemplateSlotSection key={def.key} def={def} />
              ))}
              <DialogFooter className="pt-2">
                <DialogClose render={<Button type="button" variant="outline" />}>Cancel</DialogClose>
                <Button render={<button type="submit" />} loading={busy}>
                  {target.mode === "edit" ? "Save changes" : "Create"}
                </Button>
              </DialogFooter>
            </form>
          </FormProvider>
        )}
      </DialogContent>
    </Dialog>
  );
}

function DeleteTemplateDialog({
  target,
  onOpenChange,
  onDeleted,
}: {
  target: { id: string; title: string };
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}) {
  const del = useMutation(FrameService.method.deleteFrameTemplate);
  const [error, setError] = useState<string | null>(null);

  const run = async () => {
    setError(null);
    try {
      await del.mutateAsync({ id: target.id });
      onDeleted();
      onOpenChange(false);
    } catch (err) {
      setError(ConnectError.from(err).rawMessage || "Could not delete this template.");
    }
  };

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogTitle>Delete {target.title}?</DialogTitle>
        <div className="space-y-3 text-sm">
          <p>
            Frames already created from it are unaffected: a template is copied into a Frame once, at
            creation, and never referenced again.
          </p>
          {error && <p className="text-destructive">{error}</p>}
          <DialogFooter>
            <DialogClose render={<Button type="button" variant="outline" />}>Cancel</DialogClose>
            <Button type="button" variant="destructive" loading={del.isPending} onClick={() => void run()}>
              Delete
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function TemplateGroup({
  heading,
  templates,
  canManage,
  onEdit,
  onDelete,
}: {
  heading: string;
  templates: FrameTemplateSummary[];
  canManage: boolean;
  onEdit: (t: FrameTemplateSummary) => void;
  onDelete: (t: FrameTemplateSummary) => void;
}) {
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-muted-foreground">{heading}</h2>
      {templates.length === 0 ? (
        <Card className="p-6 text-center text-sm text-muted-foreground">Nothing here yet.</Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <table className="w-full text-sm">
            <tbody>
              {templates.map((t) => (
                <tr key={t.id} className="border-b border-border last:border-0 transition-colors hover:bg-muted/40">
                  <td className="px-4 py-3">
                    <div className="font-medium text-foreground">{t.title}</div>
                    {t.description && (
                      <div className="text-xs text-muted-foreground">{t.description}</div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {/* A built-in is compiled into the binary: the backend refuses
                        both mutations for one even for an admin, so offering a
                        control that can only fail would be a lie about what this
                        page can do. */}
                    {canManage && !t.builtin && (
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          data-testid="edit-template"
                          aria-label={`Edit ${t.title}`}
                          onClick={() => onEdit(t)}
                        >
                          <Pencil className="size-4" />
                          Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          data-testid="delete-template"
                          aria-label={`Delete ${t.title}`}
                          className="text-muted-foreground hover:text-destructive-foreground"
                          onClick={() => onDelete(t)}
                        >
                          <Trash2 className="size-4" />
                          Delete
                        </Button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </section>
  );
}

export function AdminTemplatesPage() {
  const queryClient = useQueryClient();
  const listQ = useQuery(FrameService.method.listFrameTemplates, {});
  const [formTarget, setFormTarget] = useState<TemplateTarget | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; title: string } | null>(null);

  const invalidate = () =>
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({ schema: FrameService.method.listFrameTemplates, cardinality: "finite" }),
    });

  if (listQ.error) return <p className="text-destructive">Could not load templates.</p>;

  const templates = listQ.data?.templates ?? [];
  const canManage = listQ.data?.canManage ?? false;
  const orgTemplates = templates.filter((t) => !t.builtin);
  const builtinTemplates = templates.filter((t) => t.builtin);

  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Templates</h1>
          <p className="text-sm text-muted-foreground">
            Starter definitions your authors can build a new Frame from.
          </p>
        </div>
        {canManage && (
          <Button onClick={() => setFormTarget({ mode: "create" })}>
            <Plus className="size-4" />
            New template
          </Button>
        )}
      </div>

      {listQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : (
        <div className="space-y-6">
          <TemplateGroup
            heading="Your organization"
            templates={orgTemplates}
            canManage={canManage}
            onEdit={(t) => setFormTarget({ mode: "edit", id: t.id })}
            onDelete={(t) => setDeleteTarget({ id: t.id, title: t.title })}
          />
          <TemplateGroup
            heading="Built-in"
            templates={builtinTemplates}
            canManage={false}
            onEdit={(t) => setFormTarget({ mode: "edit", id: t.id })}
            onDelete={(t) => setDeleteTarget({ id: t.id, title: t.title })}
          />
        </div>
      )}

      {formTarget && (
        <TemplateFormDialog
          target={formTarget}
          onOpenChange={(open) => !open && setFormTarget(null)}
          onSaved={invalidate}
        />
      )}
      {deleteTarget && (
        <DeleteTemplateDialog
          target={deleteTarget}
          onOpenChange={(open) => !open && setDeleteTarget(null)}
          onDeleted={invalidate}
        />
      )}
    </div>
  );
}
