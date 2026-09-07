import { useState, useEffect } from "react";
import { useForm, FormProvider, useWatch, type FieldErrors } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal, X } from "lucide-react";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { authoringFormSchema, emptyFrameDoc, suggestNextVersion } from "@/lib/authoring-schema";
import { serializeFrameDoc, parseFrameContent } from "@/lib/frame-yaml";
import { SLOT_SECTIONS, sectionHasContent, type SlotSectionDef } from "@/lib/slot-sections";
import {
  seededSections,
  isRequiredSection,
  sectionHint,
  publishTemplateID,
  type TemplateRules,
} from "@/lib/templates";
import { mapPublishError } from "@/lib/publish-errors";
import { type AuthoringForm, formToDoc, docToForm } from "@/components/form/form-model";
import { ExtendsEditor } from "@/components/form/ExtendsEditor";
import { ExcludesEditor } from "@/components/form/ExcludesEditor";
import { TerminologyEditor } from "@/components/form/TerminologyEditor";
import { ListEditor } from "@/components/form/ListEditor";
import { MarkdownField } from "@/components/form/MarkdownField";
import { MarkdownSourceEditor } from "@/components/form/MarkdownSourceEditor";
import { DocMetadataHeader } from "@/components/document/DocMetadataHeader";
import { AddSectionMenu } from "@/components/document/AddSectionMenu";
import { PublishDialog } from "@/components/document/PublishDialog";
import { TemplatePicker } from "@/components/frame/TemplatePicker";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
} from "@/components/ui/dropdown-menu";
import { ResolvedPreview } from "@/components/form/ResolvedPreview";

const encode = (s: string) => new TextEncoder().encode(s);
const decode = (b: Uint8Array) => new TextDecoder().decode(b);

// One editable section of the document: heading, the editor for its content
// shape, and a remove control. Sections the author has not added simply are
// not on the page - the document editor shows the document, not the schema.
//
// `hint` overrides `def.hint` when a template's rule carries a note for this
// slot, and `removable` is false for a template-required section: deleting
// one guarantees a publish failure, so the control is withheld rather than
// offered and then punished.
function SectionEditor({
  def,
  hint,
  removable,
  onRemove,
}: {
  def: SlotSectionDef;
  hint: string;
  removable: boolean;
  onRemove: () => void;
}) {
  return (
    <section className="group space-y-2 border-t border-border pt-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{def.label}</h2>
        {removable && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="text-muted-foreground opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100"
            onClick={onRemove}
          >
            <X className="size-4" />
            Remove section
          </Button>
        )}
      </div>
      <p className="text-xs text-muted-foreground">{hint}</p>
      {def.kind === "terms" && <TerminologyEditor />}
      {def.kind === "list" && (
        <ListEditor name={def.path as `slots.${"rules" | "skills" | "prompts"}`} label={def.label} />
      )}
      {def.kind === "prose" && <MarkdownField name={def.path} />}
    </section>
  );
}

export function FrameAuthoringPage({ mode }: { mode: "create" | "edit" }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [formError, setFormError] = useState<string | null>(null);

  const methods = useForm<AuthoringForm>({
    resolver: zodResolver(authoringFormSchema),
    // A first frame is 1.0.0 unless the author says otherwise at publish time.
    defaultValues: docToForm({ ...emptyFrameDoc(), version: "1.0.0" }, ""),
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

  const templateID = searchParams.get("template") ?? "";
  // The picker is the first screen of the create flow. `?import=1` bypasses it:
  // that path already has its source document, so asking for a starting shape
  // would be nonsense. Edit mode never sees it - templates apply at creation.
  const choosingTemplate = mode === "create" && !importing && templateID === "";

  const templates = useQuery(FrameService.method.listFrameTemplates, {});
  // Fetched only once a template is chosen. Disabled otherwise so the picker
  // screen does not issue a pointless request.
  const chosen = useQuery(
    FrameService.method.getFrameTemplate,
    { id: templateID },
    { enabled: templateID !== "" },
  );
  const rules: TemplateRules = (chosen.data?.template?.fieldRules ?? {}) as TemplateRules;

  // Sections the author added this session; content-bearing sections are
  // always visible regardless (which covers the async edit-mode prefill).
  const [added, setAdded] = useState<ReadonlySet<string>>(new Set());

  const [publishOpen, setPublishOpen] = useState(false);
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

  // Seeds the form once the chosen template arrives. This mirrors the edit
  // path exactly: getFrameTemplate -> parseFrameContent -> docToForm are the
  // same three steps that open an existing Frame, so there is no second
  // parsing path to keep in step.
  const [seeded, setSeeded] = useState(false);
  useEffect(() => {
    if (mode !== "create" || templateID === "" || seeded) return;
    const tmpl = chosen.data?.template;
    if (!tmpl) return;
    try {
      // The prefill is the canonical YAML subset the backend produced, so it
      // goes through the same parse the edit flow uses.
      const doc = parseFrameContent(tmpl.prefill);
      methods.reset(docToForm({ ...doc, version: "1.0.0" }, ""));
      // Sections the template asks for are on the page from the start rather
      // than behind "+ Add section": the point of a template is that the
      // author does not have to know which sections this kind of Frame needs.
      // Unioned with whatever the prefill itself populated, so prefilled
      // content is never hidden behind a collapsed section.
      const fromRules = seededSections(rules).map((def) => def.key);
      const fromPrefill = SLOT_SECTIONS.filter((def) => sectionHasContent(def, doc.slots)).map(
        (def) => def.key,
      );
      // This is a callback reacting to an external system's data arriving
      // (the template query resolving), which is the effect rule's own
      // sanctioned use of setState-in-effect; the heuristic cannot tell that
      // apart from deriving state from other state, hence the disable.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAdded(new Set([...fromRules, ...fromPrefill]));
    } catch {
      setTimeout(() => setFormError("This template's starting content could not be loaded."), 0);
    }
    setSeeded(true);
    // seed once per template choice; re-running on every `rules` identity
    // change would fight the author's own edits after the initial seed
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chosen.data, mode, templateID, seeded]);

  const slots = useWatch({ control: methods.control, name: "slots" }) as
    | AuthoringForm["slots"]
    | undefined;
  const visibleSections = SLOT_SECTIONS.filter(
    (d) => added.has(d.key) || sectionHasContent(d, slots ?? {}),
  );
  const hiddenSections = SLOT_SECTIONS.filter((d) => !visibleSections.includes(d));

  const extendsVal = useWatch({ control: methods.control, name: "extends" }) as { ref: string }[] | undefined;
  const hasParents = (extendsVal ?? []).some((e) => e.ref?.trim());

  // Org slug for post-publish navigation. PublishFrameResponse carries orgId,
  // not the slug, and the detail route is keyed by slug, so read it from GetMe.
  const meQ = useQuery(FrameService.method.getMe, {});
  const publish = useMutation(FrameService.method.publishFrame);
  // One RPC backs both editor directions, import, and export, so the slot
  // table lives only in Go rather than being mirrored again in TypeScript.
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

  const addSection = (def: SlotSectionDef) => {
    setAdded((prev) => new Set(prev).add(def.key));
  };
  const removeSection = (def: SlotSectionDef) => {
    // Clearing the value is what removes a content-bearing section; the set
    // only tracks intentionally-added empty ones.
    methods.setValue(def.path as never, (def.kind === "prose" ? "" : []) as never, { shouldDirty: true });
    methods.clearErrors(def.path as never);
    setAdded((prev) => {
      const next = new Set(prev);
      next.delete(def.key);
      return next;
    });
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
  const onSubmit = async (form: AuthoringForm) => {
    setFormError(null);
    const content = encode(serializeFrameDoc(formToDoc(form)));
    try {
      await publish.mutateAsync({
        content,
        changelog: form.changelog,
        // publishTemplateID owns this rule (create only, and only when a
        // template was actually chosen) and is unit-tested on its own; the
        // page must call it rather than re-implement the condition inline,
        // or the two can drift.
        templateId: publishTemplateID({ mode, importing, templateID }),
      });
      afterPublish(form.name);
    } catch (err) {
      const { fieldErrors, formError: fe } = mapPublishError(err);
      for (const [path, message] of Object.entries(fieldErrors)) {
        methods.setError(path as never, { type: "server", message });
      }
      setFormError(fe);
      if (!fieldErrors.version) setPublishOpen(false);
    }
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
  // publish path: the server only ever stores the canonical slot YAML.
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

  if (choosingTemplate) {
    return (
      <TemplatePicker
        templates={templates.data?.templates ?? []}
        // The choice goes in the URL rather than component state, so it is
        // linkable and survives a reload, matching how `?import=1` works.
        onPick={(id) => setSearchParams({ template: id }, { replace: true })}
      />
    );
  }

  return (
    <FormProvider {...methods}>
      <form onSubmit={(e) => e.preventDefault()} className="mx-auto max-w-3xl space-y-6">
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
              <DropdownMenu>
                <DropdownMenuTrigger variant="ghost" aria-label="More actions">
                  <MoreHorizontal className="size-4" />
                </DropdownMenuTrigger>
                <DropdownMenuPortal>
        <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={toMarkdown}>Edit as Markdown</DropdownMenuItem>
                  <DropdownMenuItem disabled={!hasParents} onClick={() => setPreviewOpen(true)}>
                    Preview as resolved Frame
                  </DropdownMenuItem>
                </DropdownMenuContent>
      </DropdownMenuPortal>
              </DropdownMenu>
            )}
            <Button type="button" variant="outline" onClick={() => navigate(-1)}>Cancel</Button>
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
          <div className="space-y-6">
            <DocMetadataHeader nameReadOnly={mode === "edit"} />

            {/* Composition sits between metadata and content, the way the
                frontmatter it maps to sits above the document body. */}
            <div className="space-y-3 rounded-md border border-border bg-card p-4">
              <div className="space-y-2">
                <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Inherits from</h3>
                <ExtendsEditor />
              </div>
              <div className="space-y-2">
                <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Excludes</h3>
                <ExcludesEditor />
              </div>
            </div>

            {visibleSections.map((def) => (
              <SectionEditor
                key={def.key}
                def={def}
                hint={sectionHint(rules, def)}
                removable={!isRequiredSection(rules, def.key)}
                onRemove={() => removeSection(def)}
              />
            ))}

            <div className="border-t border-border pt-4">
              <AddSectionMenu available={hiddenSections} onAdd={addSection} />
            </div>
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
          version={methods.getValues("version")}
          open={previewOpen}
          onClose={() => setPreviewOpen(false)}
        />
      </form>
    </FormProvider>
  );
}
