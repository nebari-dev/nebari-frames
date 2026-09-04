import { type FrameDoc, DEFAULT_VISIBILITY } from "@/lib/frame-yaml";

export interface AuthoringForm extends FrameDoc {
  changelog: string;
}

// Strips rows the user left blank so client + server agree on emptiness.
export function formToDoc(form: AuthoringForm): FrameDoc {
  const doc: FrameDoc = {
    name: form.name,
    description: form.description,
    version: form.version,
    visibility: form.visibility,
    scope: (form.scope ?? "").trim(),
    maintainer: (form.maintainer ?? "").trim(),
    template: form.template ?? false,
    body: form.body ?? "",
  };

  const extendsFiltered = (form.extends ?? []).filter(
    (e) => e.ref.trim() !== "",
  );
  if (extendsFiltered.length > 0) doc.extends = extendsFiltered;

  const excludesFiltered = (form.excludes ?? []).filter(
    (x) => x.trim() !== "",
  );
  if (excludesFiltered.length > 0) doc.excludes = excludesFiltered;

  return doc;
}

export function docToForm(doc: FrameDoc, changelog: string): AuthoringForm {
  // Frames published before `visibility` existed carry none. Seed the spec
  // default rather than showing an empty required field, so opening an old
  // frame for editing does not present an error the author did not cause.
  return { ...doc, visibility: doc.visibility || DEFAULT_VISIBILITY, changelog };
}
