import { useState, useEffect } from "react";
import { useForm, FormProvider, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate, useParams } from "react-router";
import { useMutation, useQuery, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { authoringFormSchema, emptyFrameDoc, suggestNextVersion } from "@/lib/authoring-schema";
import { serializeFrameDoc, parseFrameContent } from "@/lib/frame-yaml";
import { mapPublishError } from "@/lib/publish-errors";
import { type AuthoringForm, formToDoc, docToForm } from "@/components/form/form-model";
import { MetadataFields } from "@/components/form/MetadataFields";
import { ExtendsEditor } from "@/components/form/ExtendsEditor";
import { ExcludesEditor } from "@/components/form/ExcludesEditor";
import { TerminologyEditor } from "@/components/form/TerminologyEditor";
import { ListEditor } from "@/components/form/ListEditor";
import { MarkdownField } from "@/components/form/MarkdownField";
import { ChangelogField } from "@/components/form/ChangelogField";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { ResolvedPreview } from "@/components/form/ResolvedPreview";

const PROSE: { name: `slots.${string}`; label: string }[] = [
  { name: "slots.tool_specs", label: "Tool Specifications" },
  { name: "slots.goals", label: "Goals" },
  { name: "slots.style", label: "Style" },
  { name: "slots.norms", label: "Norms" },
  { name: "slots.architecture", label: "Architecture" },
  { name: "slots.business_process", label: "Business Process" },
];

export function FrameAuthoringPage({ mode }: { mode: "create" | "edit" }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [formError, setFormError] = useState<string | null>(null);

  const methods = useForm<AuthoringForm>({
    resolver: zodResolver(authoringFormSchema),
    defaultValues: docToForm(emptyFrameDoc(), ""),
  });

  const [previewOpen, setPreviewOpen] = useState(false);
  const { org = "", name = "" } = useParams();
  const editQ = useQuery(
    FrameService.method.getFrame,
    { orgSlug: org, name },
    { enabled: mode === "edit" },
  );

  useEffect(() => {
    if (mode !== "edit" || !editQ.data?.version) return;
    try {
      const doc = parseFrameContent(editQ.data.version.content);
      methods.reset(docToForm({ ...doc, version: suggestNextVersion(doc.version) }, ""));
    } catch {
      // Schedule outside the effect body to satisfy react-hooks/set-state-in-effect
      setTimeout(() => setFormError("This frame's content could not be loaded for editing."), 0);
    }
    // reset only when the loaded version changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, editQ.data?.version?.digest]);

  const extendsVal = useWatch({ control: methods.control, name: "extends" }) as { ref: string }[] | undefined;
  const hasParents = (extendsVal ?? []).some((e) => e.ref?.trim());

  // Org slug for post-publish navigation. PublishFrameResponse carries orgId,
  // not the slug, and the detail route is keyed by slug, so read it from GetMe.
  const meQ = useQuery(FrameService.method.getMe, {});
  const publish = useMutation(FrameService.method.publishFrame);

  const isDirty = methods.formState.isDirty && !publish.isSuccess;
  // In-app SPA route-change blocking would require migrating to a data router
  // (createBrowserRouter + RouterProvider); deferred. The beforeunload handler
  // below covers browser-level navigation (tab close / refresh / hard nav) per
  // the design doc's "browser confirmation" requirement.

  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { if (isDirty) e.preventDefault(); };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isDirty]);

  const onSubmit = (form: AuthoringForm) => {
    setFormError(null);
    const doc = formToDoc(form);
    const content = new TextEncoder().encode(serializeFrameDoc(doc));
    publish.mutate(
      { content, changelog: form.changelog },
      {
        onSuccess: () => {
          queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({
              schema: FrameService.method.listFrames,
              cardinality: "finite",
            }),
          });
          const slug = mode === "edit" ? org : (meQ.data?.org?.slug ?? "");
          navigate(`/frames/${slug}/${form.name}`);
        },
        onError: (err: unknown) => {
          const { fieldErrors, formError: fe } = mapPublishError(err);
          for (const [path, message] of Object.entries(fieldErrors)) {
            methods.setError(path as never, { type: "server", message });
          }
          setFormError(fe);
        },
      },
    );
  };

  return (
    <FormProvider {...methods}>
      <form onSubmit={methods.handleSubmit(onSubmit)} className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">{mode === "create" ? "New Frame" : "Edit Frame"}</h1>
          <div className="flex gap-2">
            <Button type="button" variant="outline" disabled={!hasParents} onClick={() => setPreviewOpen(true)}>Preview as resolved Frame</Button>
            <Button type="button" variant="outline" onClick={() => navigate(-1)}>Cancel</Button>
            {/* base-ui Button defaults to type="button"; the render prop wins mergeProps precedence, so set submit on the rendered element */}
            <Button render={<button type="submit" />} disabled={publish.isPending}>Publish</Button>
          </div>
        </div>
        {formError && <Alert variant="destructive">{formError}</Alert>}

        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Metadata</h2><MetadataFields nameReadOnly={mode === "edit"} /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Inherits from</h2><ExtendsEditor /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Excludes</h2><ExcludesEditor /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Terminology</h2><TerminologyEditor /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Rules</h2><ListEditor name="slots.rules" label="Rules" /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Skills</h2><ListEditor name="slots.skills" label="Skills" /></section>
        <section className="space-y-2"><h2 className="text-sm font-semibold uppercase text-muted-foreground">Prompts</h2><ListEditor name="slots.prompts" label="Prompts" /></section>
        {PROSE.map((p) => <section key={p.name}><MarkdownField name={p.name} label={p.label} /></section>)}
        <section><ChangelogField /></section>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={() => navigate(-1)}>Cancel</Button>
          {/* base-ui Button defaults to type="button"; the render prop wins mergeProps precedence, so set submit on the rendered element */}
          <Button render={<button type="submit" />} disabled={publish.isPending}>Publish</Button>
        </div>

        <ResolvedPreview
          org={org}
          name={methods.getValues("name")}
          version={methods.getValues("version")}
          open={previewOpen}
          onClose={() => setPreviewOpen(false)}
        />
      </form>
    </FormProvider>
  );
}
