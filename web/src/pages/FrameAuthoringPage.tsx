import { useState, useEffect, useId, useRef } from "react";
import { useForm, FormProvider, useWatch, type FieldErrors } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { authoringFormSchema, emptyFrameDoc, suggestNextVersion } from "@/lib/authoring-schema";
import { serializeFrameDoc, parseFrameContent } from "@/lib/frame-yaml";
import { FRAME_TEMPLATES } from "@/lib/frame-templates";
import { mapPublishError } from "@/lib/publish-errors";
import { type AuthoringForm, formToDoc, docToForm } from "@/components/form/form-model";
import { ExtendsEditor } from "@/components/form/ExtendsEditor";
import { ExcludesEditor } from "@/components/form/ExcludesEditor";
import { MarkdownField } from "@/components/form/MarkdownField";
import { MarkdownSourceEditor } from "@/components/form/MarkdownSourceEditor";
import { DocMetadataHeader } from "@/components/document/DocMetadataHeader";
import { PublishDialog } from "@/components/document/PublishDialog";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ResolvedPreview } from "@/components/form/ResolvedPreview";

const encode = (s: string) => new TextEncoder().encode(s);
const decode = (b: Uint8Array) => new TextDecoder().decode(b);

export function FrameAuthoringPage({ mode }: { mode: "create" | "edit" }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();
  const [formError, setFormError] = useState<string | null>(null);

  const methods = useForm<AuthoringForm>({
    resolver: zodResolver(authoringFormSchema),
    // A first frame is 1.0.0 unless the author says otherwise at publish time.
    // ?template=1 (the admin Templates page's "New template") pre-checks the
    // offer-as-a-template box.
    defaultValues: docToForm(
      { ...emptyFrameDoc(), version: "1.0.0", template: searchParams.get("template") === "1" },
      "",
    ),
  });

  // "Import a .frame.md" lands straight in the Markdown editor: it is the
  // import surface, so there is no separate mapping screen to keep in step
  // with the codec. Otherwise Markdown is a secondary mode reached via the
  // overflow menu, never a persistent preference.
  const importing = mode === "create" && searchParams.get("import") === "1";
  const [editorMode, setEditorMode] = useState<"document" | "markdown">(
    importing ? "markdown" : "document",
  );
  const [markdownSource, setMarkdownSource] = useState("");
  const [markdownErrors, setMarkdownErrors] = useState<string[]>([]);

  // The starter template last applied, so switching templates before writing
  // anything replaces cleanly, while switching after edits asks first.
  // A "@org/name" id sources the template from an existing frame; ?from= is
  // the "Use as template" entry point on a frame's detail page.
  const initialFrom = mode === "create" ? (searchParams.get("from") ?? "") : "";
  const [templateId, setTemplateId] = useState(initialFrom ? `@${initialFrom}` : "");
  const [fromRef, setFromRef] = useState(initialFrom);
  const appliedTemplateBody = useRef("");

  const [publishOpen, setPublishOpen] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  // The template picker's visible text label sits outside the trigger, so the
  // trigger points at it with aria-labelledby.
  const templatePickerLabelId = useId();
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
  // One RPC backs both editor directions, import, and export, so the .frame.md
  // codec lives only in Go rather than being mirrored again in TypeScript.
  const convert = useMutation(FrameService.method.convertFrame);

  const busy = publish.isPending || convert.isPending;

  const isDirty = (methods.formState.isDirty || markdownSource !== "") && !publish.isSuccess;
  // In-app SPA route-change blocking would require migrating to a data router
  // (createBrowserRouter + RouterProvider); deferred. The beforeunload handler
  // below covers browser-level navigation (tab close / refresh / hard nav) per
  // the design doc's "browser confirmation" requirement.

  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { if (isDirty) e.preventDefault(); };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isDirty]);

  // Frames flagged as templates in this org, offered alongside the built-ins.
  const orgTemplatesQ = useQuery(
    FrameService.method.listFrames,
    {},
    { enabled: mode === "create" && !importing },
  );
  const orgTemplates = (orgTemplatesQ.data?.frames ?? []).filter((f) => f.isTemplate);
  // ?from= may name a frame that is not flagged; it still needs a picker entry.
  const fromOutsideCatalog =
    fromRef !== "" && !orgTemplates.some((f) => `${f.orgSlug}/${f.name}` === fromRef);

  const [fromOrg = "", fromName = ""] = fromRef.split("/");
  const fromQ = useQuery(
    FrameService.method.getFrame,
    { orgSlug: fromOrg, name: fromName },
    { enabled: mode === "create" && fromName !== "" },
  );

  // A frame-sourced template applies when its content arrives.
  useEffect(() => {
    if (!fromRef || !fromQ.data?.version) return;
    try {
      const doc = parseFrameContent(fromQ.data.version.content);
      appliedTemplateBody.current = doc.body;
      methods.setValue("body", doc.body, { shouldDirty: true });
      if (doc.scope && (methods.getValues("scope") ?? "").trim() === "") {
        methods.setValue("scope", doc.scope, { shouldDirty: true });
      }
    } catch {
      setTimeout(() => setFormError("The selected template's content could not be loaded."), 0);
    }
    // apply once per selected source version
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fromRef, fromQ.data?.version?.digest]);

  // Applying a template replaces the content (and fills an empty scope). Edits
  // the author has made since the last template are never discarded silently.
  const applyTemplate = (id: string) => {
    const current = methods.getValues("body") ?? "";
    if (
      current.trim() !== "" &&
      current !== appliedTemplateBody.current &&
      !window.confirm("Replace the current content with the template?")
    ) {
      return;
    }
    setTemplateId(id);
    if (id.startsWith("@")) {
      setFromRef(id.slice(1)); // content applies when the frame loads
      return;
    }
    setFromRef("");
    const tpl = FRAME_TEMPLATES.find((t) => t.id === id);
    const body = tpl?.body ?? "";
    appliedTemplateBody.current = body;
    methods.setValue("body", body, { shouldDirty: true });
    if (tpl?.scope && (methods.getValues("scope") ?? "").trim() === "") {
      methods.setValue("scope", tpl.scope, { shouldDirty: true });
    }
  };

  // Collects violation messages for the markdown editor. Structural failures
  // arrive on "markdown"; value failures (an unpinned inherits ref) arrive on
  // their form path and are shown with that path so they stay actionable.
  const showViolations = (err: unknown) => {
    const { fieldErrors, formError: fe } = mapPublishError(err);
    const messages = Object.entries(fieldErrors).map(([path, msg]) =>
      path === "markdown" ? msg : `${path}: ${msg}`,
    );
    setMarkdownErrors(messages);
    setFormError(messages.length > 0 ? null : fe);
  };

  const toMarkdown = () => {
    setFormError(null);
    const yaml = serializeFrameDoc(formToDoc(methods.getValues()));
    convert.mutate(
      { source: { case: "yaml", value: encode(yaml) } },
      {
        onSuccess: (res) => {
          setMarkdownSource(decode(res.markdown));
          setMarkdownErrors([]);
          setEditorMode("markdown");
        },
        onError: (err) => setFormError(mapPublishError(err).formError ?? "Could not render this frame as Markdown."),
      },
    );
  };

  // Markdown is a superset view of the document, so the parse must succeed
  // before switching back; on failure the author stays in Markdown with the
  // errors annotated.
  const toForm = () => {
    setFormError(null);
    convert.mutate(
      { source: { case: "markdown", value: encode(markdownSource) } },
      {
        onSuccess: (res) => {
          try {
            const doc = parseFrameContent(res.yaml);
            methods.reset(docToForm(doc, methods.getValues("changelog")));
            setMarkdownErrors([]);
            setEditorMode("document");
          } catch {
            setMarkdownErrors(["The converted frame could not be loaded into the form."]);
          }
        },
        onError: showViolations,
      },
    );
  };

  const afterPublish = (publishedName: string) => {
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({
        schema: FrameService.method.listFrames,
        cardinality: "finite",
      }),
    });
    const slug = mode === "edit" ? org : (meQ.data?.org?.slug ?? "");
    navigate(`/frames/${slug}/${publishedName}`);
  };

  // A publish failure closes the dialog only when the problem lives outside
  // it: a version conflict must be fixed where the version input is.
  const onSubmit = (form: AuthoringForm) => {
    setFormError(null);
    const content = encode(serializeFrameDoc(formToDoc(form)));
    publish.mutate(
      { content, changelog: form.changelog },
      {
        onSuccess: () => afterPublish(form.name),
        onError: (err: unknown) => {
          const { fieldErrors, formError: fe } = mapPublishError(err);
          for (const [path, message] of Object.entries(fieldErrors)) {
            methods.setError(path as never, { type: "server", message });
          }
          setFormError(fe);
          if (!fieldErrors.version) setPublishOpen(false);
        },
      },
    );
  };

  // Invalid form on publish: keep the dialog open only when the version itself
  // is the problem; otherwise close it so the inline errors are visible.
  const onInvalid = (errors: FieldErrors<AuthoringForm>) => {
    if (!errors.version) {
      setPublishOpen(false);
      setFormError("Fix the highlighted fields, then publish again.");
    }
  };

  // Publishing from Markdown converts first so there is still exactly one
  // publish path: the server only ever stores the canonical YAML.
  const publishFromMarkdown = () => {
    setFormError(null);
    convert.mutate(
      { source: { case: "markdown", value: encode(markdownSource) } },
      {
        onSuccess: (res) => {
          publish.mutate(
            { content: res.yaml, changelog: methods.getValues("changelog") },
            {
              onSuccess: () => {
                let published = methods.getValues("name");
                try {
                  published = parseFrameContent(res.yaml).name;
                } catch {
                  // Fall back to the form's name; the server accepted the doc.
                }
                afterPublish(published);
              },
              onError: (err) => {
                setPublishOpen(false);
                showViolations(err);
              },
            },
          );
        },
        onError: (err) => {
          setPublishOpen(false);
          showViolations(err);
        },
      },
    );
  };

  const confirmPublish = () => {
    if (editorMode === "markdown") publishFromMarkdown();
    else void methods.handleSubmit(onSubmit, onInvalid)();
  };

  const title = mode === "edit" ? "Edit Frame" : importing ? "Import Frame" : "New Frame";

  return (
    <FormProvider {...methods}>
      <form
        onSubmit={(e) => e.preventDefault()}
        className="flex min-h-0 flex-1 flex-col gap-6"
      >
        <div className="flex items-center justify-between">
          <h1 className="text-sm font-medium uppercase tracking-wide text-muted-foreground">{title}</h1>
          <div className="flex items-center gap-2">
            {editorMode === "markdown" ? (
              !importing && (
                <Button type="button" variant="ghost" disabled={busy} onClick={toForm}>
                  Back to editor
                </Button>
              )
            ) : (
              <Button type="button" variant="ghost" disabled={busy} onClick={toMarkdown}>
                Edit as Markdown
              </Button>
            )}
            <Button type="button" variant="outline" onClick={() => navigate(-1)}>Cancel</Button>
            {editorMode !== "markdown" && (
              <Button
                type="button"
                variant="outline"
                disabled={!hasParents}
                title="Preview this Frame with its inherited parents merged in"
                onClick={() => setPreviewOpen(true)}
              >
                Preview resolved
              </Button>
            )}
            <Button type="button" disabled={busy} onClick={() => setPublishOpen(true)}>
              Publish&hellip;
            </Button>
          </div>
        </div>
        {formError && <Alert variant="destructive">{formError}</Alert>}

        {editorMode === "markdown" ? (
          <MarkdownSourceEditor
            value={markdownSource}
            onChange={(v) => {
              setMarkdownSource(v);
              setMarkdownErrors([]);
            }}
            errors={markdownErrors}
            busy={busy}
          />
        ) : (
          <div className="grid min-h-0 gap-x-10 gap-y-6 lg:flex-1 lg:grid-cols-[minmax(0,32rem)_minmax(0,1fr)] lg:items-stretch">
            {/* Left: identity, spec metadata, and composition as a standard
                labeled form column. */}
            <div className="space-y-6">
              <DocMetadataHeader nameReadOnly={mode === "edit"} />

              <div className="space-y-4 border-t border-border pt-4">
                <div className="space-y-1.5">
                  <h3 className="text-sm font-medium text-foreground">Inherits from</h3>
                  <ExtendsEditor />
                </div>
                <div className="space-y-1.5">
                  <h3 className="text-sm font-medium text-foreground">Excludes</h3>
                  <ExcludesEditor />
                </div>
              </div>
            </div>

            {/* Right: the frame's content, the main editing surface. */}
            <section className="flex min-h-0 flex-col gap-2">
              <div className="flex items-center justify-between gap-4">
                <h2 className="text-lg font-semibold">Content</h2>
                {mode === "create" && (
                  <div className="flex items-center gap-2">
                    <span
                      id={templatePickerLabelId}
                      className="text-xs font-medium text-muted-foreground"
                    >
                      Start from a template
                    </span>
                    <Select value={templateId} onValueChange={(v) => applyTemplate(String(v))}>
                      <SelectTrigger
                        aria-labelledby={templatePickerLabelId}
                        className="h-8 w-48 text-xs"
                      >
                        <SelectValue placeholder="Blank" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">Blank</SelectItem>
                        <SelectGroup>
                          <SelectLabel>Starter templates</SelectLabel>
                          {FRAME_TEMPLATES.map((t) => (
                            <SelectItem key={t.id} value={t.id} title={t.hint}>
                              {t.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                        {(orgTemplates.length > 0 || fromOutsideCatalog) && (
                          <SelectGroup>
                            <SelectLabel>Org templates</SelectLabel>
                            {fromOutsideCatalog && (
                              <SelectItem value={`@${fromRef}`}>{fromName}</SelectItem>
                            )}
                            {orgTemplates.map((f) => (
                              <SelectItem
                                key={`${f.orgSlug}/${f.name}`}
                                value={`@${f.orgSlug}/${f.name}`}
                                title={f.description}
                              >
                                {f.name}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        )}
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                Free-form Markdown: the context this Frame carries into AI conversations.
                Keep it concise — include only guidance that would change how work is done.
              </p>
              {/* Fills whatever height is left below the page chrome,
                  matching the view page's panel. */}
              <MarkdownField
                name="body"
                ariaLabel="Content"
                fill
                className="min-h-[20rem]"
                placeholder="The rules, terminology, goals, style, or process this Frame exists to convey."
              />
            </section>
          </div>
        )}

        <PublishDialog
          open={publishOpen}
          onOpenChange={setPublishOpen}
          onConfirm={confirmPublish}
          pending={busy}
          versionFromSource={editorMode === "markdown"}
        />

        <ResolvedPreview
          org={org}
          name={methods.getValues("name")}
          // Empty = latest published version. The form's version field holds
          // the next (unpublished) version, which the resolver can't know.
          version=""
          open={previewOpen}
          onClose={() => setPreviewOpen(false)}
        />
      </form>
    </FormProvider>
  );
}
