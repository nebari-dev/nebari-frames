# Design Doc: Frame Spec v0.3 - a domain model, encoding bindings, and a conformance validator

| | |
|---|---|
| **Status** | Draft - pending review |
| **Author** | Chuck McAndrew |
| **Created** | 2026-09-07 |
| **Last updated** | 2026-09-07 |
| **Reviewers** | TBD (proposal targets `openteams-ai/frame-spec`) |
| **Amended** | 2026-09-07, after the draft was written and reviewed: D13 and D14 added; section 4 and the model diagram updated to match openteams-ai/frame-spec#28. |
| **Aligned against** | *Intelligence Hub Whitepaper* Revision 9 (`openteams-ai/inthub-whitepaper` at `9c28569`, 17 August 2026): sections 2, 3.3, 4.2 to 4.5, 5, 5.6, 6.1, 7.3 to 7.5, 8, and `GLOSSARY.md`; checked against the v8 PDF (6 August 2026) first, then the Revision 9 text and the v8-to-Revision-9 diff. *OpenTeams Open Source Product Strategy* v5 (20 August 2026): goals 1, 5, 6 and the execution priorities. |

## TL;DR

[Frame Spec v0.2.0](https://github.com/openteams-ai/frame-spec) does not specify a Frame. It specifies a Markdown file convention: four mandatory frontmatter keys, a free-form body, and one orphaned rule about inheritance precedence. The Frame *model* exists in that repository only as non-normative prose in `docs/overview.md`, which PR #25 fenced off with a disclaimer rather than reconciling.

Nebari Frames implemented that working definition as its ten-slot schema. The spec then narrowed away from it. The measurable result is one-way conformance: our exported `.frame.md` documents pass the spec project's own validator 4 of 4, while the spec project's own example Frames fail our importer 18 of 18.

This document proposes **Frame Spec v0.3**: an abstract Frame model, a defined element set with obligations and crosswalks to recognized metadata standards, composition rules attached to the element that carries them, three encoding bindings (Markdown, YAML, JSON), and a reference validator that checks all three against a machine-readable profile.

The design is grounded in existing standards rather than invented. The document skeleton is the DCMI **Singapore Framework**; the abstract-artifact-versus-serialization split is DCAT's `Dataset`/`Distribution` pattern; the machine-readable profile is **DCTAP**; individual elements crosswalk to DC Terms, schema.org, PROV-O, SKOS, and ADMS. One relation, composition, has no equivalent in any of them and is declared Frame-native with a written rationale.

v0.2's text is not edited. It becomes the Markdown encoding binding, and the compatibility claim the validator is built to prove is that **every valid v0.2 Frame is a valid v0.3 Frame**.

## 1. Problem

### 1.1 What v0.2 actually specifies

Read element by element, almost everything normative in v0.2 is a fact about a Markdown file rather than a fact about a Frame.

| Element of v0.2 | What it specifies | Layer |
|---|---|---|
| Conformance rule | "A file is a valid v0.2 Frame if it is a Markdown file, and its YAML frontmatter carries the four required fields." Conformance is predicated on a file, so nothing that is not a file can conform. | serialization |
| `type` | A magic number. Its stated purpose is telling a reader that a `.md` file "is intended to be handled as a Frame rather than as generic Markdown", a problem only a filesystem has. | serialization |
| `visibility` | Required, with "suggested values", no defined meaning, no actor model, no stated effect. | undefined |
| `name`, `description` | Defined by prose length and readability. Nothing says whether `name` identifies or labels. | undefined |
| `inherits` | "Typically a file path, a Frame name, or a URI. The spec does not require a specific resolution mechanism." | undefined |
| Body content | The payload is explicitly unspecified: "no required sections, no expected sections, and no section taxonomy at all." | undefined |
| Expected Agent Handling | Four steps of parsing one encoding. | serialization |
| Reference validator | A line-based frontmatter linter, explicitly "not a full YAML parser". | serialization |
| Inheritance precedence | Explicit declaration, child wins, parents in order, non-transitive by default. | **model** |

The last row is the exception and it demonstrates the problem rather than softening it. Those precedence rules are genuine serialization-independent semantics, and they cannot be implemented consistently because the references they operate on have no identity and no resolution model. Collab resolves `inherits` not at all ([frame-spec#21](https://github.com/openteams-ai/frame-spec/issues/21)); Nebari Frames demands `org/name@version` resolved transitively. Both readings are conformant.

### 1.2 We are the same shape

This cuts at Nebari Frames too. We do not store Markdown. `proto/frames/v1/frame.proto` stores `bytes content // full YAML document`, and our normative artifact is a Go struct with `yaml.KnownFields(true)`. So `frame-spec` describes a Markdown binding and Nebari Frames describes a YAML binding. Neither repository describes the Frame.

With no model to reconcile against, every disagreement has had to be litigated between two parsers, and a parser argument has no principled resolution.

### 1.3 Measured conformance

| Direction | Result |
|---|---|
| `nebari-frames/examples/*.frame.md` against `frame-spec/tools/validate_frames.py` | 4 of 4 pass |
| `frame-spec/examples/**/*.md` (18 frontmatter-bearing files) against `frames.UnmarshalMarkdown` | 0 of 18 accepted |

Representative rejections:

```
# frame-spec/examples/code-review-norms/frame.md
# (the example PR #25 added to show that four fields and a plain body is enough)
REJECT  line 10: content before the first section heading; frame body content
        must sit under a recognized "## " section

# frame-spec/examples/sow-review/*.frame.md - all nine lenses
REJECT  line 9: unknown frontmatter key "status"

# frame-spec/examples/minimal-self-frame/frame.md
REJECT  unknown section "## Purpose" - did you mean "## Goals"?
        unknown section "## Ways Of Working" - did you mean "## Norms"?
        malformed terminology entry - expected "- **term**: definition"
```

## 2. Goals and non-goals

### 2.1 Goals

1. A Frame model that a registry, a desktop application, and a file on disk can all conform to.
2. Element definitions with obligations, drawn from recognized metadata standards wherever a term already exists.
3. Composition rules precise enough that two implementations produce the same output, or declare why they differ.
4. Three encoding bindings, with v0.2 preserved as the Markdown one.
5. A reference validator covering all three encodings, driven by the same machine-readable profile the prose describes.
6. Backward compatibility: every valid v0.2 Frame is a valid v0.3 Frame.

### 2.2 Non-goals

| | Non-goal | Rationale |
|---|---|---|
| N1 | Grants, roles, revocation, effective permissions | The strategy's goal 1 places these inside "one public Frame protocol." This design does not: they are registry semantics, and putting them in the artifact model is what would make the spec un-adoptable for Collab. So the Frame protocol the strategy names resolves to **two documents**: this spec, and a registry contract that is not yet scoped (open question 4). Goal 1 is half-met by this spec alone. That is stated here rather than redefined away. |
| N2 | Packaging and distribution (Nebi, OCI) | Distribution carries Frames; it does not define them. `docs/nebi-integration.md` remains exploratory. |
| N3 | Review workflow, scoring, approval gates | `status` exposes enough state for tools to implement a workflow. The spec should not be a workflow engine. |
| N4 | Guard, Gate, Track, Cog and Op contracts | Separate artifacts with separate contracts. A Frame's *reference* to a Guard is in scope (`guards`, section 6.3); what a Guard is, how it runs, and how the reference resolves are the Guard contract's to define. |
| N5 | The full `v1-gap-analysis.md` field set | That document is explicitly future-facing. Adopting it wholesale is the failure mode PR #25 was written to prevent. |
| N6 | Editing v0.2's text | It becomes the Markdown binding as written. Preserving it is correct on the merits, and it respects the direction the stewards set in PR #25 rather than reopening it. |
| N7 | Section-level visibility | The whitepaper names it three times (section 4.3 "reviewed subsets", the External Vendor Frame example, section 7.5 "selective field-level controls"), so deferring it is a scoping choice, not an oversight. Declared section-level intent needs an additive model change (an optional visibility qualifier on refinement values), and only its enforcement depends on N1. The refinement structure is the hook: it gives sections an identity to attach a qualifier to. The strategy's Frame protocol list does not name it, so it is deferred to keep the first release small. |
| N8 | Directory form and nested Frames | The whitepaper's definition says "a file or folder of files" and adds "nestable" to the property list. v0.2 explicitly defers the directory form, and this design inherits that deferral rather than silently omitting it. When it is taken up, `dcterms:hasPart` / `dcterms:isPartOf` are the crosswalk terms, and `Representation` is where a folder-shaped serialization attaches without a model change. |

## 3. Standards basis

### 3.1 Why a crosswalk rather than one parent standard

There is no recognized metadata standard for AI context artifacts. The domain-native standard is [AGENTS.md](https://agents.md/), now stewarded by the Agentic AI Foundation under the Linux Foundation and reported in use by over 60,000 open-source projects, and it deliberately has no schema at all: "AGENTS.md is just standard Markdown. Use any headings you like; the agent simply parses the text you provide." SKILL.md is two required YAML fields plus free Markdown.

That validates keeping a Frame's body free-form. It also shows why AGENTS.md cannot serve as a registry artifact: no identity, no version, no maintainer, no access intent, no composition. A Frame is an AGENTS.md-shaped body inside a specified metadata envelope, and the envelope is where a recognized standard earns its keep.

No single standard covers identity, versioning, lineage, and content typing. Rather than force one, this design does what DCAT, ADMS and RO-Crate themselves did: reuse existing terms and declare a profile. Credibility is then established per element by crosswalk, not by fiat.

One naming note for the upstream proposal. The whitepaper and the strategy say "Frame protocol" throughout; `frame-spec`'s own stewardship Frame says "Prefer `Frame spec` over `Frame protocol`." This document uses "spec" for the artifact definition and reserves "protocol" for the strategy's broader sense in section 13. The two communities should settle on one word before the proposal lands.

### 3.2 Documents referenced

| Standard | Used for | Reference |
|---|---|---|
| DCMI Singapore Framework | The five-component document skeleton | <https://www.dublincore.org/specifications/dublin-core/singapore-framework/> |
| DCMI Metadata Terms | Element definition structure; several element mappings | <https://www.dublincore.org/specifications/dublin-core/dcmi-terms/> |
| DCMES 1.1 | Definition plus Comment convention per element | <https://www.dublincore.org/specifications/dublin-core/dces/> |
| DC Usage Guide | One-to-one principle; dumb-down principle; encoding schemes | <https://www.dublincore.org/specifications/dublin-core/usageguide/> |
| DCAP guidelines | What a profile may and may not do to a borrowed term | <https://www.dublincore.org/specifications/dublin-core/profile-guidelines/> |
| DCTAP | The machine-readable profile table | <https://www.dublincore.org/specifications/dctap/elements/> |
| DCAT 3 | `Dataset`/`Distribution` split; versioning properties | <https://www.w3.org/TR/vocab-dcat-3/> |
| schema.org CreativeWork | `creativeWorkStatus`, `maintainer`, `conditionsOfAccess` | <https://schema.org/CreativeWork> |
| PROV-O | Derivation and revision lineage | <https://www.w3.org/TR/prov-o/> |
| SKOS | Terminology values as concepts | <https://www.w3.org/TR/skos-reference/> |
| RFC 2119 / BCP 14 | Requirement keywords, and the test for using them | <https://www.rfc-editor.org/rfc/rfc2119> |

### 3.3 Two rules borrowed with teeth

**The DCAP constraint on borrowed terms.** Profile designers "may add technical constraints on use of properties (such as repeatability), or provide more narrow interpretations of definitions for particular purposes, but they should not contradict the meaning of the properties intended by their maintainers."

This is a precise diagnosis of our `name` divergence. Constraining `name` to `[a-z0-9-]` does not narrow "a short human-readable name"; it contradicts it. Legal profile moves are now distinguishable from illegal ones by citation.

**RFC 2119 section 6.** Requirement keywords "MUST only be used where it is actually required for interoperation or to limit behavior which has potential for causing harm."

Applied to v0.2, `visibility` fails this test: required, undefined, and with no interoperation role. Applied to v0.3, it is the test that sets the obligation floor in section 5.1.

## 4. Deliverables

One specification document, a companion profile, and a validator. The document is written in the structure of an IETF Internet-Draft (introduction, conventions, data model, elements, composition, encodings, conformance profiles, implementation status, security considerations, IANA considerations, references, appendices) as plain GitHub-flavored Markdown, so that it renders where the repository is read. The Singapore Framework remains the completeness checklist the document is checked against, not its published form. A kramdown-rfc source that renders to xml2rfc text and HTML with zero warnings is retained outside the repository for the day the draft is submitted (D13).

| Path | Content | Normative |
|---|---|---|
| `spec/frame-spec.md` | The working draft: the Internet-Draft structure above, with the three encodings as sections 6.2 to 6.4 and usage guidance folded into each element's Comment | **Yes** once released; this is the working-draft slot the repository already defines |
| `spec/profile/frame-core.csv` | The element set as a DCTAP profile, byte-identical to the draft's Appendix B | **Yes**, with the draft |
| `tools/validate_frame.py` | Reference validator for all three encodings, driven from the profile | No, but conformance-defining in practice |

Opened as openteams-ai/frame-spec#28 together with an Apache-2.0 LICENSE and alignment of the repository's other documents. The sections below describe the design; the draft is the reference where they differ.

### 4.1 Profile columns

`frame-core.csv` uses DCTAP's own columns (`propertyID`, `propertyLabel`, `mandatory`, `repeatable`, `valueNodeType`, `valueDataType`, `valueConstraint`, `valueConstraintType`, `note`) plus two Frame-specific ones, since DCTAP defines no way to express either:

- `mapsTo` - the crosswalked term URI, or empty for a Frame-native element. Journey 2 checks that an empty value is always paired with a rationale in `note`.
- `refines` - the element this one refines, which carries the dumb-down relation from section 6.2 into the machine-readable profile.

Extra columns are consistent with DCTAP's intent: it defines a minimum set and expects profiles to be read by tools that ignore what they do not recognize. The two additions are declared in the profile's own header row and documented in the draft's Appendix B.

`spec/v0.2.md` stays frozen and untouched. Section 6.2 of the draft is the Markdown encoding; it restates v0.2's requirements and cites v0.2 as normative.

## 5. Domain model

Four things, shaped on DCAT because its `Dataset`/`Distribution` split is this problem exactly: "This distinction allows one dataset to have multiple distributions in different formats."

```
Frame                        the abstract artifact; persists across versions
  identifier, title, description, maintainer, scope, visibility,
  license, canonicalSource
  |
  +-- derivedFrom            (relation)                  [prov:wasDerivedFrom]
  |     this Frame was adapted or forked from that one
  |
  +-- FrameVersion           a revision                  [dcat:hasVersion]
        version, status, versionNotes, issued             prov:wasRevisionOf
        guidance + refinements                            dcat:previousVersion
        |
        +-- composition      (relation) ordered references to other
        |                    Frames; the declaring Frame has precedence.
        |                    Frame-native: no standard term means this
        +-- guards           (relation) Guards to run on output  [dcterms:requires]
        |
        +-- Representation   a serialization             [dcat:Distribution]
              mediaType, checksum, byteSize               spdx:checksum
                                                          dcat:byteSize
```

`composition` and `guards` sit on the Frame Version because they can differ between revisions; `derivedFrom` sits on the Frame because lineage belongs to the Frame. This placement was corrected during the draft's review; the draft's Figure 1 is the reference.


Three consequences worth stating explicitly in the spec:

**`Representation` dissolves an existing confusion.** Our stored slot-YAML and our exported `.frame.md` are two Representations of one FrameVersion: same Frame, same version, different `mediaType` and `checksum`. Today we have no way to express that, which is why "which one is the real Frame?" keeps recurring. The answer is neither; the FrameVersion is.

**`type` leaves the model.** Under this shape the class *is* `Frame`, so the `type: frame [0.2]` token is a file-sniffing magic number and belongs in the Markdown binding alone. This retroactively justifies our verifying it on import and never storing it.

**One-to-one principle, applied.** Per the DC Usage Guide, a Frame description describes the Frame: not the subject the Frame is about, and not the organization maintaining it. Stated explicitly because the failure mode is real, and `scope: company` invites people to describe the company.

### 5.1 Obligation floor

RFC 2119 section 6 sets the test. For system A to hand a Frame to system B and have B behave correctly, B must know **which Frame this is** and **what it says**. Nothing else qualifies.

- `identifier` MUST be present.
- `guidance` MUST be present, and MAY be empty.
- Everything else is optional at the model layer, with obligation declared per profile.

`guidance` MAY be empty because v0.2 never required body content, and the compatibility claim in section 2.1 would otherwise fail for a v0.2 Frame with frontmatter and nothing under it. Presence is structural: every encoding has a content position, so it always exists even when its value does not. A Frame with empty `guidance` is valid and useless, which is the correct outcome for a spec to permit and a validator to note.

The Markdown binding profile keeps v0.2's four keys mandatory, so v0.2 and v0.3-Markdown are compatible in both directions: existing Frames stay valid, and new Markdown Frames stay v0.2-valid. `visibility` stops being a model-level requirement it could never justify, without invalidating a single existing file.

`identifier` is satisfiable for a bare file because each encoding binding specifies that a Frame with no explicit identifier is identified by its retrieval location, which is how both RO-Crate and AGENTS.md already work. A Frame emailed as a file is identified by that file.

The strategy's end-to-end path has Nebi "assign and resolve the artifact identity" and publish by digest, and says Nebi "should carry their common identity, version, dependencies, provenance, registry location, compatibility, and lifecycle without becoming the only place their meaning can be defined." That is a second identity at the envelope layer, and the two must not be confused. `identifier` is the Frame's own claim of identity and travels inside the artifact. A distribution mechanism such as Nebi or a registry MAY assign its own artifact identity in its envelope, SHOULD record the Frame's `identifier` alongside it, and is bound by the identity rules in section 6.5 if it changes the Frame-level identifier. `canonicalSource` is where a Frame can point at its authoritative envelope location (a `nebi://` or registry URL) without the model depending on any one distribution system.

## 6. Element set

Each element in the spec carries Name, Label, Definition, Comment, Obligation, Repeatable, and Maps-to, following the DCMES Definition-plus-Comment convention.

### 6.1 Identity and description

| Element | Obl. | Rep. | Maps to | Comment summary |
|---|---|---|---|---|
| `identifier` | MUST | no | `dcterms:identifier` | Identity of the Frame, stable across versions and copies. Not the version's identity. |
| `title` | SHOULD | no | `dcterms:title`, `schema:name` | Human-readable. **Implementations MUST NOT constrain it to an identifier syntax.** |
| `description` | SHOULD | no | `dcterms:description`, `schema:abstract` | No length limit. One or two sentences recommended. |
| `version` | SHOULD | no | `dcat:version`, `schema:version` | The Frame's own revision, not the spec version. SemVer recommended. |
| `versionNotes` | MAY | yes | `adms:versionNotes` | What changed. Our `changelog`. |
| `status` | MAY | no | `schema:creativeWorkStatus` | Registered values (SHOULD): `draft`, `review`, `approved`, `deprecated`, `revoked`. Unregistered values are preserved, never rejected (D14). |
| `maintainer` | SHOULD | yes | `schema:maintainer`; secondary `schema:accountablePerson` (partial) | Definition is an exact match, and PR #20's `author` to `maintainer` rename already landed here. The maintainer is also the accountable party the whitepaper's "Owned" property requires: a Frame is "owned by and accountable to a human or a group of humans that intentionally manage it," and the Comment says so. `accountablePerson` is partial because its range is Person only, and a maintainer may be a team. |
| `scope` | MAY | no | `dcterms:audience` (partial) | Where the Frame applies. Mapping is flagged partial rather than forced. |
| `visibility` | MAY | no | `dcterms:accessRights`, `schema:conditionsOfAccess` | Declared intent. **MUST NOT be treated as an access control.** Registered values (SHOULD): `private`, `internal`, `shared`, `public`, which v0.2 listed as suggested. Unregistered values are preserved (D14). |
| `license` | MAY | no | `dcterms:license` | Cheap to carry, and cross-organization exchange needs it. |
| `issued` | MAY | no | `dcterms:issued` | When this version was published. Our `published_at`. ISO 8601. |
| `canonicalSource` | MAY | no | `schema:sameAs`, `prov:specializationOf` | The authoritative location of this Frame, disambiguating identifiers that are unique only within one registry. Defined now so identifiers published today are not ambiguous later; **not emitted by Nebari Frames until a second registry exists to disambiguate against.** |

The `title` row is the `name` divergence fixed with teeth: a slug constraint on `title` becomes explicitly non-conformant, cited to the DCAP rule in section 3.3.

`canonicalSource` maps to `schema:sameAs` ("URL of a reference Web page that unambiguously indicates the item's identity") rather than the `dcat:accessURL` first floated in review: `accessURL` is a Distribution-level property and the wrong layer. `prov:specializationOf` is the secondary mapping for the case where a local copy is a specialization of the canonical Frame.

In a single document the Frame-level and FrameVersion-level elements of section 5 collapse into one set. The distinction matters to registries, which MAY treat Frame-level elements as shared across versions; a file carries one version's view.

### 6.2 Content

| Element | Obl. | Rep. | Maps to |
|---|---|---|---|
| `guidance` | MUST | yes | Frame-native (nearest: `schema:text`) |

`guidance` is the body: "Context that orients work performed within the Frame's scope." Neither `schema:text` nor `dcterms:description` means instructional context for work, so the nearest term is recorded without claiming equivalence.

Ten optional refinements, each `refines: guidance`, all MAY, all repeatable, with definitions lifted from the whitepaper's section 4.3 list, which `docs/overview.md` reproduces (the whitepaper's eleventh item, Output Guards, is the `guards` relation in section 6.3, not a refinement):

`rules`, `terminology`, `goals`, `style`, `norms`, `skills`, `toolSpecs`, `prompts`, `architecture`, `businessProcess`

The whitepaper's wording for tool specifications is "Nebi (or similar) spec files that document the tools the Frame expects to be available," so `toolSpecs` values are typically references to environment specifications rather than prose. The Comment says so. As content, they still degrade to `guidance` under dumb-down.

Every refinement's value is content, exactly as `guidance` is. A refinement MAY additionally define a structured form that a reader MAY extract; `terminology` defines one, where an entry of the form *term: definition* is a `skos:Concept` with `skos:prefLabel` and `skos:definition`. Content under a refinement that does not fit its structured form is still that refinement's content, and a reader MUST NOT reject it or demote it.

This is not a hypothetical. Three of the spec repo's own 18 examples, including `minimal/frame.md`, carry a `## Terminology` section whose bullets are usage preferences (`Prefer "Frame" over "alignment file".`) rather than term/definition pairs. Our current codec rejects all three as "malformed terminology entry". Under this rule they are valid `terminology` content that happens to contain no extractable concepts.

> **Dumb-down rule (normative).** A reader that does not implement a refinement MUST treat its value as `guidance`. It MUST NOT discard it. A reader that implements a refinement but cannot extract its structured form MUST keep the content as that refinement's value.

This is the DC Usage Guide's principle applied directly: "A client should be able to ignore any qualifier and use the value as if it were unqualified." A Frame with zero refinements is fully valid, satisfying PR #25 absolutely. A Frame with all ten degrades to an AGENTS.md-shaped body for any reader that ignores them. The free-form body and the typed slots become one Frame seen at two resolutions.

### 6.3 Composition and lineage

| Element | Obl. | Rep. | Maps to | Meaning |
|---|---|---|---|---|
| `composition` | MAY | yes, **ordered** | Frame-native | A Frame whose guidance composes with this one's, this one taking precedence |
| `derivedFrom` | MAY | yes | `prov:wasDerivedFrom`, `prov:hadPrimarySource` | This Frame was adapted or forked from that one |
| `previousVersion` | MAY | no | `dcat:previousVersion`, `prov:wasRevisionOf` | Version lineage |
| `guards` | MAY | yes | `dcterms:requires` | A Guard that must be run on output produced under this Frame |

`guards` is the fourth relation, and both source documents require it. The whitepaper's content list (v8 and Revision 9 alike) has eleven categories, not ten; the eleventh is "Output Guards: validation tools to be run to verify the AI produces what is intended." Section 4.3 adds: "One of the critical things that Frames can do is define a Validation or Verification tool (a Guard) that must be called and pass on the output of the system." Section 5: "Frames can attach a validation code that must be run." Section 5.1: "Frames can declare associated Guards." The strategy names "stable Guard references" in the Frame protocol list twice. It is a metadata-level relation rather than a refinement because a Guard is run on output, not read as context; under dumb-down a refinement would degrade into prose loaded into the model, which is the wrong failure mode for a validation requirement. Its value is a reference in rule 9's grammar, and resolution is deferred until a Guard contract exists, exactly as `composition` defers resolution of Frame references. `dcterms:requires` ("a related resource that is required by the described resource to support its function, delivery, or coherence") is the crosswalk. Unlike the other relations, `guards` composes: see rule 5.

v0.2 collapsed three distinct relations into one field. `composition` is not derivation: `isBasedOn` and `wasDerivedFrom` mean "this was made from that", while composition means "this combines with that at activation". Mapping composition to either would contradict the borrowed term's meaning, which section 3.3 forbids. It is therefore declared Frame-native with that rationale written into the Comment. For v0.2 compatibility, the Markdown binding encodes it as `inherits`.

### 6.4 Representation level

Present only in encoding bindings and registry profiles, never in a Frame's own content: `mediaType` (`dcat:mediaType`), `checksum` (`spdx:checksum`), `byteSize` (`dcat:byteSize`).

**Cut in #28.** All three were removed during review, and the Representation entity now defines no element. Each describes a stored copy rather than the Frame, no encoding carried any of them, and a reader was required to accept them yet forbidden to act on them while a writer would never emit one. The entity stays in the model, because it is what distinguishes the same Frame Version written as Markdown from the same one written as JSON, and the element registry keeps a Representation level for future use. The element count is therefore 27, not 30.

### 6.5 Identity rules

Two normative rules attach to `identifier`, both cheap and both preventing a silent fork.

> A reader that assigns an identifier to a Frame which arrived without one MUST NOT present that identifier as the Frame's own claim.
>
> A reader that changes a Frame's identifier MUST record the prior value in `derivedFrom`.

Consider importing a Frame that already carries `identifier: openteams/brand-voice` into org `acme`. Without these rules it would be stored as `acme/brand-voice` and the exchange record would show nothing about where it came from. With them, `derivedFrom: openteams/brand-voice` is mandatory, and `derivedFrom` acquires a job in the first release instead of being decorative.

The minting policy itself (what a registry derives a fresh identifier from) is a profile concern, not a model one. Nebari Frames' profile: slugify `title`, and on collision reject with a message naming the conflict rather than suffixing, because `-2` produces identifiers nobody can predict or reference. Filename is not a candidate: `frame-spec`'s own convention is `frame.md` inside a named directory, so every import would collide on `frame`.

### 6.6 Extensibility

> **Extensibility rule (normative).** Readers MUST preserve unknown elements and MUST NOT reject a Frame for carrying them. Implementation-specific elements take an `x-` prefix.

This settles two divergences at the model layer instead of per-parser: the example files in `frame-spec/examples` carrying an undefined `status` (9 of the 18 Markdown Frames, and 11 files across the whole examples tree once the YAML ones are counted), and our `x-nebari-excludes`. It also makes round-trips lossless by construction.

`x-nebari-excludes` (subtractive composition: compose with a parent but drop a named ancestor) stays an extension because only one implementation wants it. If a second one does, it is a candidate for promotion to a model element.

### 6.7 Element count

27 Frame-level elements: 2 mandatory (`identifier`, `guidance`), 10 optional refinements, and 15 optional metadata and relation elements. Plus 3 representation-level elements that never appear in a Frame's own content. For comparison, DCMES 1.1 has 15 elements, all optional.

## 7. Composition rules

Per the brief, these attach to the `composition` element rather than floating in a chapter of their own. Rules 1 through 4 are v0.2's, carried over unchanged.

1. **Explicit.** A Frame composes only what it declares.
2. **Ordered.** Earlier entries have lower precedence than later entries.
3. **Child highest.** The declaring Frame takes precedence over all composed Frames.
4. **Transitivity is OPTIONAL.** Implementations MUST declare their behavior in their conformance profile.
5. **Only content elements and `guards` compose.** `guidance` and its refinements compose. `guards` accumulates: a Guard declared by any Frame in the composed set applies to the result, so a compliance Frame's Guard cannot be composed away by a child that does not mention it. The elements that describe the Frame itself (`identifier`, `title`, `description`, `version`, `versionNotes`, `status`, `maintainer`, `scope`, `visibility`, `license`, `issued`, `canonicalSource`, `derivedFrom`, `previousVersion`) MUST NOT be inherited from a composed Frame.
6. **Resolution follows repeatability.** For repeatable content elements, composition concatenates in precedence order. For non-repeatable content elements, the highest-precedence value replaces.
7. **Unresolvable references.** A reader that resolves composition MUST NOT silently ignore a reference it cannot resolve. It MUST fail or report. A reader that resolves no composition at all MUST declare so in its conformance profile and SHOULD surface the presence of unresolved composition to the user.
8. **Cycles.** A reader MUST detect cycles and MUST NOT loop.
9. **Reference syntax.** The model mandates none. The grammar classifies references as `<publisher>/<name>@<version>`, `<uri>`, `<path>`, or otherwise `<name>`; **any non-empty string is at least a `<name>`**, so reference syntax is never grounds for rejecting a Frame. Readers declare which forms they resolve.

Rule 5's `guards` clause follows the whitepaper's own example: a Healthcare Compliance Frame "automatically incorporated into any Cog touching patient data, and pointing to a Guard that must be run after every output." If a child could drop that Guard by omission, the compliance Frame would not do what it exists to do. The `frame-spec` sketch had the same instinct for rules: "Required rules from broader scopes should not disappear silently." Excluding a Frame from composition entirely (this repository's `x-nebari-excludes`) removes its Guards along with its content, which is consistent.

Rule 5 exists, for the metadata elements, because without it a child that omits `description` would inherit its parent's, which is wrong and which neither implementation does. Our resolver already carries the child's own metadata through untouched, on the grounds that "spec metadata describes the child itself and is never inherited"; this rule promotes that behavior from an implementation detail to a model rule.

Rule 6 is the load-bearing addition. Merge semantics stop needing a separate specification and fall out of precedence plus the DCTAP `repeatable` column. Our per-slot merge and Collab's bare concatenation both become conformant and, for the first time, comparable. This answers [nebari-frames#62](https://github.com/nebari-dev/nebari-frames/issues/62) with a table column.

Rule 6 admits two profile narrowings, both of which MUST be declared: deduplication of identical values within a repeatable element, and key-based replacement within one (our `terminology` merge, where a child's definition of a term replaces a parent's). For `terminology` the second is not even a narrowing: SKOS S14 requires that a concept scheme carry at most one `skos:prefLabel` per language, so replace-by-term falls out of the crosswalk. A profile MUST NOT drop non-identical values under either narrowing.

Rule 7's second sentence exists so that rule 4 and rule 7 do not conflict. Collab's current behavior (`inherits` silently ignored) is non-conformant as it stands and becomes conformant only by declaring it. That is the intended effect. An author relying on layering deserves to know.

Rule 9 is permissive by design. The grammar exists so readers can say which forms they resolve, not so validators can reject Frames; a strict grammar would fail valid v0.2 Frames whose `inherits` values are arbitrary strings.

**Session composition.** The whitepaper's "Composable" property is not inheritance: "Multiple Frames can be combined for a given work session ... company + department + project + ad-hoc context as the work requires." Those Frames need not declare each other. The `frame-spec` gap analysis called sibling composition "the largest spec gap." Rules 2, 3, 5 and 6 apply to any ordered set of Frames an implementation combines, whether the order comes from a `composition` declaration or from activation-time selection by a user or application; in the second case the activation order is the precedence order, later activated wins, and the rules are otherwise unchanged. The spec states this so that the desktop application's session layering and a registry's resolved inheritance are governed by the same rules.

**Declared variation versus uniform interpretation.** The whitepaper's trust argument (section 8.1) wants "a Frame inherited by one Cog will be interpreted the same way by another." This design guarantees identical interpretation of content (dumb-down forbids content loss) and identical precedence, and it permits declared variation in resolution depth (rule 4) and merge narrowing (rule 6). Full uniformity would require making transitive resolution mandatory, which PR #25 made optional and which Collab does not do. The tradeoff is stated rather than hidden: an author who needs identical behavior across tools consults the conformance profiles, which exist so the variation is visible.

## 8. Usage guidelines

Informative. Per-element guidance plus:

- **One-to-one principle**, with the `scope: company` failure mode as the worked example.
- **Dumb-down principle**, restated with a worked example of a ten-refinement Frame read by a core-only reader.
- **Value encoding schemes**: SemVer for `version`, ISO 8601 for `issued`, SKOS for `terminology`, and the two closed vocabularies for `status` and `visibility`.
- **Conciseness** and the **system-context trust note**, both carried from v0.2 verbatim.
- **When not to use a refinement**, preserving PR #25's line: a Frame that carries a single rule well is a good Frame.

## 9. Encoding bindings

Each binding states how every element is encoded, the identifier default, how unknown elements are preserved, and round-trip requirements.

### 9.1 Markdown

YAML frontmatter plus Markdown body. Frontmatter keys are element names, with two aliases for v0.2 compatibility: `name` for `title`, `inherits` for `composition`. Requires v0.2's four keys (`type`, `name`, `description`, `visibility`).

Compatibility runs both ways. v0.2 documents are valid v0.3-Markdown because the four keys are still required and `guidance` may be empty. v0.3-Markdown documents pass v0.2's reference validator because v0.2 neither prohibits additional frontmatter keys nor does `validate_frames.py` reject them, and its `type` pattern (`^frame(?: \[\d+\.\d+\])?$`) accepts `frame [0.3]`. Verified during design review: a Markdown Frame carrying `type: frame [0.3]`, `identifier`, `status`, `license`, `issued`, `derivedFrom`, `canonicalSource`, and `x-nebari-excludes` passes `validate_frames.py` (1 checked, 1 passed). Readers MUST accept any `frame [M.N]` token and MAY warn on a version they do not know.

> A `##` heading whose text matches a refinement label is that refinement. **Every other heading, and all loose prose, is `guidance`.**

That rule is the whole 18-of-18 fix. An unrecognized heading stops being a parse error and becomes content. It does not reintroduce a body taxonomy: no heading is required, unrecognized headings are content rather than errors, and a v0.2 reader sees ordinary Markdown. It only lets a v0.3-aware reader recover structure that is already there in how people write.

Binding details the validator depends on:

- Heading match is case-insensitive on the label, ignoring surrounding whitespace. The ten labels are `Rules`, `Terminology`, `Goals`, `Style`, `Norms`, `Skills`, `Tool Specifications`, `Prompts`, `Architecture`, `Business Process`. `Rules of the Game` is not a match.
- Sub-headings (`###` and deeper) inside a refinement section belong to that refinement.
- The binding emits **one** `guidance` value per document, containing all non-refinement body content in document order with its own headings preserved. `guidance` being repeatable matters for composition, not for a single file.
- A `terminology` bullet of the form `- **term**: definition` is an extractable concept, mapped to `skos:prefLabel` and `skos:definition`. Any other content under `## Terminology` is kept as unstructured `terminology` content (section 6.2). The binding never fails on the shape of a bullet.
- Qualified labels are deliberately not matched. The examples contain `## Primary Goals`, `## Review Norms`, `## Delivery Norms`, `## Client Relationship Norms` and four more of that shape, eight in all across two files; all are `guidance`. Suffix matching would silently relocate an author's content, which is worse than leaving it where they put it. The old codec's heading hints (`ways of working` to Norms, `constraints` to Rules) are likewise not applied automatically.
- Round-trip preserves element values, not source layout. A `## Rules` section sitting between two chunks of loose prose is extracted, and the prose is joined; re-emission places refinement sections after `guidance`. Authors who care about layout stability SHOULD put refinement sections last.

### 9.2 YAML

A mapping of element names to values, with `guidance` and the refinements as top-level keys. `type` MAY be present and MUST NOT be required; a structured document needs no file-sniffing token.

This is *structurally close* to Nebari Frames' stored form, not identical to it. Differences the import work must absorb: `name` splits into `identifier` and `title`; `extends` becomes `composition`; `excludes` becomes `x-nebari-excludes`; the `slots` nesting flattens to top-level refinements; and **our storage has no `guidance` at all**, which is the one element the model requires. `terminology` items keep our `{term, definition}` shape, with an optional `altTerms` list, and the three keys map to `skos:prefLabel`, `skos:definition`, and `skos:altLabel`.

### 9.3 JSON

As YAML, plus an optional JSON-LD `@context` mapping element names to the crosswalked URIs from section 6, and `@type: Frame`. The crosswalk pays for itself here: a Frame becomes consumable as linked data at no additional cost. `--encoding json` is the one validator mode with no dependency beyond the standard library.

## 10. Validator design

`tools/validate_frame.py`. The architecture deliberately mirrors the spec's own structure, which makes it a check on the design: if the validator cannot be built this way, the model and the bindings are not actually separable.

```
   markdown front end  \
   yaml front end       >--->  normalized element model  --->  profile check  --->  report
   json front end      /              (section 5)          (frame-core.csv)
```

- **Front ends** parse one encoding each and emit the same normalized element model.
- **Profile check** loads `spec/profile/frame-core.csv` and validates obligation, repeatability, and value constraints from it. Nothing about the element set is hard-coded, so prose, profile and validator cannot drift.
- **Checks**: required elements present; registered vocabularies checked, with unregistered values warned about and preserved, never failed; unknown elements reported as preserved, never failed; composition references *classified* by form (rule 9) and never failed on syntax; refinement content that lacks its structured form reported as unstructured, never failed (section 6.2); cycles detected across a set of Frames when more than one is supplied.
- **Dependencies**: PyYAML is optional. Without it the Markdown and YAML front ends fall back to the line-based frontmatter parser that `validate_frames.py` already contains, with a warning that YAML structure is unverified. This keeps the zero-install property the current tool has, while letting `pip install pyyaml` turn on real validation. Journeys 1, 6 and 7 are run with PyYAML present.
- **Round-trip mode** (`--round-trip`) re-encodes the normalized model into each other encoding and diffs the result.
- **Self-check mode** (`--self-check`) verifies the CSV against the spec prose, which is journey item 3.

CLI:

```
validate_frame.py [paths...] [--encoding auto|markdown|yaml|json]
                  [--profile PATH] [--round-trip] [--self-check]
```

## Journeys

| # | Item | Proof | Check method | Evidence |
|---|------|-------|--------------|----------|
| 1 | Every valid v0.2 Frame is a valid v0.3 Frame | All 18 frontmatter-bearing `.md` files in `frame-spec/examples` pass validation, including the 9 carrying `status: stable`, an unregistered value that must be preserved, the 2 with relative-path `inherits`, the 3 whose `## Terminology` bullets are not term/definition pairs, and the 2 whose bodies carry qualified headings such as `## Review Norms` (8 such headings) that must land in `guidance` | automated: `validate_frame.py frame-spec/examples/` exits 0 with 18 passed, 0 failed | **verified 2026-09-09** at frame-spec `9f3a53c`: `validate_frame.py examples --quiet` reports `Frames checked: 18   passed: 18   failed: 0   skipped: 12`, exit 0. The nine `status: stable` Frames each warn once and pass. Independently reproduced by the reviewer's own grep of the corpus. |
| 2 | No element is invented without justification | Every element carries either a resolvable crosswalk URI or a written rationale for being Frame-native; zero silent gaps | automated: `validate_frame.py --self-check` fails on any element whose Maps-to is empty and whose Comment lacks a native-term rationale | **verified 2026-09-09**: `--self-check` reports `27 elements agree between CSV and draft`, exit 0, comparing label, obligation, repeatability, level, crosswalk term and registered values. The count fell from 30 when `mediaType`, `checksum` and `byteSize` were cut: they describe a Representation, not a Frame, and no encoding carried them. Note the check as first written could not fire for the ten refinements, which were hard-coded as Frame-native; that was found and fixed before this evidence was taken. Crosswalk term *resolvability* is deliberately not tested, since dereferencing would make CI depend on third-party vocabulary hosting. |
| 3 | The machine-readable profile matches the prose | Every element in section 6 appears in `frame-core.csv` with identical obligation and repeatability, and every CSV row appears in the prose | automated: `validate_frame.py --self-check` | **verified 2026-09-09**: same `--self-check` run, plus a new `appendix-mismatch` check confirming Appendix B's embedded copy of the profile is byte-identical to `spec/profile/frame-core.csv`. Element count established independently of the checker's own patterns, by extracting section 4.7's summary table and comparing against the CSV: 27 both ways, symmetric difference empty. The profile also gained a `level` column, so the section 10.2 registry can be built from the file, and the checker now compares that column against the same table; mutating the CSV's `guidance` row to `Frame` reports `level-mismatch`. |
| 4 | An implementer can build a conformant reader from the spec alone | A reader written by an implementer with no access to this conversation, working only from the spec files, (a) accepts all 18 examples, (b) rejects a Markdown Frame that omits the required `name` key, and rejects one whose `type` fails the `frame` sentinel while accepting with a warning one whose version token is merely a shape it does not recognize (criterion amended 2026-09-09, see below), (c) preserves `status: bogus` with at most a warning rather than rejecting it, and (d) given a Frame with a `## Rules` section, exposes that content as either `rules` or `guidance` and never drops it | narrated: dispatch a clean-context implementer agent, capture its reader and its run output against the four cases | **PARTIALLY VERIFIED 2026-09-09.** A fresh agent, forbidden from reading `tools/` and not told how many examples exist, built a reader from `spec/frame-spec.md` and `frame-core.csv` alone. (a) PASS: 18 found, 18 accepted, 0 rejected. (b) PASS **after the draft was amended to resolve the contradiction it found**: the missing-`name` document is rejected, `type: framework` is rejected because the sentinel fails, and `type: frame [0.3.0]` is accepted with a `nonstandard-type-version` warning by both readers. Before the amendment the independent reader accepted it and our validator rejected it, each following a different MUST. (c) PASS: `status: bogus` preserved verbatim. (d) PASS: `## Rules` content landed under `rules`, quoting section 6.2.2. It recorded 13 ambiguities. The most consequential is unstated refinement cardinality: our reader splits a refinement section per bullet, theirs treats the section as one value, and on `examples/minimal/frame.md` this yields `goals` as two values versus one containing both bullets and their markup. Two conforming readers, materially different models for the same document. Report at `~/devel/frame-spec-implementer/report.md`. **VERIFIED 2026-09-09, second build.** After the twenty ambiguities that report and this one's own review surfaced were settled in the draft, a second agent, in a worktree holding the revised draft and no implementation, built a reader from `spec/frame-spec.md` and `frame-core.csv` alone. It accepted 18 of 18 and dumped a model per example; those dumps are at `~/devel/frame-spec-reader-2/models/`, its reader at `reader.py`, its crafted cases at `crafted/`. Compared element by element against this implementation's model for the same 18 files today: **18 agree, 0 disagree.** Two things are excluded and both are accounted for. The location-derived `identifier` differs because the two working copies sit at different paths, and no example states an identifier of its own, so nothing else could be compared there. `guidance` came back as a scalar from their reader and a one-item list from our model; section 6.3 has since required a scalar in the structured encodings, and our writer emits one in both, so the shapes now agree in every encoding and the list is internal only. Criteria (a) through (d) all pass. |
| 5 | Composition is unambiguous enough that two readers agree | A three-Frame chain has exactly one output predicted by the rules, written into the spec as a worked example: at model level, `rules` concatenates in precedence order (rules 2, 3, 6) and a parent's `description` is not inherited (rule 5); under a profile declaring `style` non-repeatable, the child's `style` replaces (rule 6, replace branch) | automated: fixtures under `spec/fixtures/composition/` with expected output for the model-level and profile-level cases | **verified 2026-09-09** at model level and under a profile: `--compose` over the three fixtures in precedence order diffs byte-identical against `expected-model.json`, and again under `spec/profiles/nebari-frames.yaml` against `expected-style-non-repeatable.json`. Caveat recorded honestly: implementing these rules surfaced five ambiguities in section 5.1, so the fixtures pin one reading rather than prove the rules admit only one. |
| 6 | Round-trips are lossless across encodings | A Frame with all ten refinements, a `guards` reference, and an `x-` extension element survives markdown to json to yaml to markdown with every element value identical | automated: `validate_frame.py --round-trip` on the fixture | **verified 2026-09-09**: `--round-trip spec/fixtures/roundtrip/full.frame.md` reports `ROUND-TRIP OK`; the fixture carries all ten refinements, a `guards` reference and an `x-` extension, each confirmed present through the parser rather than by reading the file. All 18 examples also round-trip clean. Known structural loss: alternative labels on a terminology concept cannot be expressed in Markdown, so the writer folds them into the bullet text under the dumb-down rule. That limitation was not true as first written, since front matter would have carried them in a nested mapping; section 6.2.1 now defines a front matter value as a scalar or a sequence of scalars, and a nested value is preserved and warned about rather than read as a structured form. |
| 7 | The validator supports every documented encoding | The same Frame authored as `.md`, `.yaml` and `.json` normalizes to structurally identical element models (deep-equal) | automated: three-encoding fixture, normalized-model comparison | **verified 2026-09-09**: the three-encoding fixture's parsed models are deep-equal, 2 tests pass, and the fixtures directory validates 3 of 3. All three fixture files were confirmed byte-identical to the draft's own figures. The equivalence once held only under our reading of refinement cardinality; section 6.2.2 now states the rule, so it holds under the specification rather than under a choice, which is journey 4's finding closed. |
| 8 | Both implementations' real behavior is expressible as a profile | Filled-in conformance profiles for Nebari Frames (transitive; pinned refs only; six refinements narrowed to non-repeatable; key-replace on `terminology`; slugified-title minting) and Collab (resolves nothing, declared per rule 7) both validate against the template and differ visibly. The template requires a declaration for every rule the model leaves to profiles: 4, 6's narrowings, 7, 9, and the identity rules in 6.5 | narrated: both profiles written, diff captured | **verified 2026-09-09**: `--check-profile` reports both profiles complete, exit 0, and they differ visibly on resolution, reference forms, narrowings and identifier minting. The Nebari profile reproduces the composition fixture byte-identically, and the reviewer confirmed it traces clause by clause to section 8 and Appendix C rather than being reverse-engineered to pass that diff. The Collab profile is framed as observed behavior citing issue 21, which I read to confirm it does not overstate. |
| 9 | `visibility` finally means something | The element carries a Definition, a Comment stating it is not an access control, and a registered vocabulary: the three things v0.2 never gave it | narrated: section read, quoted into evidence | **verified 2026-09-09**: section 4.3.8 now carries a Definition ("The sharing boundary the maintainer declares for the Frame"), the four registered values, a MUST to preserve an unregistered value and never reject for one, and the sentence "It is not an access control, and readers MUST NOT treat it as one" pointing at section 9. v0.2 required the key and defined nothing. |

### Journey drift, declared 2026-09-09

Two journey criteria were written before the specification was amended, and the
amendments came from what the journeys measured. Recording the drift rather than
leaving the block looking as though it always said this.

Journey 4's criterion (b) expected a conforming reader to reject `type: frame [0.3.0]`.
Sections 6.2.1 and 6.2.2 have since been amended, so that criterion is superseded: the
`frame` sentinel MUST match and a version token of an unrecognized shape now warns.
The criterion above is rewritten to the amended rule and the evidence retaken. This is
the journey working: it detected a contradiction between two MUSTs, the specification
resolved it, and both readers now agree.

Journey 4's headline finding, unstated refinement cardinality, is also now settled in
section 6.2.2: cardinality follows the element's repeatability, a repeatable section
yields one value per top-level block, and a conformance profile's non-repeatable
narrowing governs composition only and MUST NOT change how a document is read. The
reference implementation already behaved this way, which is why no code changed for it.
The wording needed one correction of its own: the first draft said non-list content was
one further value, which the implementation contradicts by yielding a value per
paragraph, so it now says one value per top-level block.

### Verification status, 2026-09-09

**9 of 9 journey items verified.** Journey 4 passed on a second, independent build after the specification was amended to settle what the first build's divergences exposed. Its original partial result and both divergences are kept above and below rather than overwritten.

The validator, its fixtures and both conformance profiles are built, reviewed and pushed
as `spec/v0.3-validator` in `openteams-ai/frame-spec`, 63 signed commits, 214 tests, with
continuous integration green. Every row's evidence above was captured fresh at that branch.

Journey 4's divergences were defects in this specification, not in the reader that found
them. All of them are now settled in the draft; each paragraph below keeps what was found
and states how it resolved, rather than being deleted.

**Two requirements contradict each other.** Section 6.2.1 says the `type` value MUST be
`frame` or `frame [<major>.<minor>]`. Section 6.1 says readers MUST accept a token naming
any version and MAY warn on one they do not recognize. A document declaring
`frame [0.3.0]` names a version and breaks the grammar, so one reader rejects it and
another accepts it with a warning, each following a MUST. The draft does not say which
wins. **Resolved:** section 6.2.1 separates the sentinel from the version token. The word
`frame` MUST match, case-sensitively, and a version token of an unrecognized shape or
version is accepted with a warning. Both readers now agree on all three cases.

**Refinement cardinality is unstated, and it changes the model.** Section 4.2.2 says that
in a single document `guidance` has one value and is repeatable only so that composition
can combine several Frames. No parallel sentence exists for the ten refinements, and only
`terminology` is given an explicit item-level split, in section 6.2.3. So a reader may
treat a refinement section as one value or as one value per bullet, and both readings
follow the text. On `examples/minimal/frame.md` this is the difference between

    goals = ["Be clear, direct, and credible.", "Avoid hype and overclaiming."]
    goals = ["- Be clear, direct, and credible.\n- Avoid hype and overclaiming."]

This is the most consequential gap the exercise found. It also reaches journey 7: the
three-encoding equivalence holds under the per-bullet reading, because the YAML and JSON
fixtures write each rule as its own sequence entry, and would not hold under the
one-value-per-section reading. **Resolved:** section 6.2.2 now ties cardinality to the element's
repeatability. A repeatable section yields one value per top-level block, each item of a
top-level list is a value, the marker is not part of it, and a profile's non-repeatable
narrowing governs composition only and MUST NOT change how a document is read. Journey 7's
equivalence therefore holds under the specification rather than under a choice of
reading.

Eleven further ambiguities are recorded in the independent reader's report, including no
Markdown syntax for a terminology concept's alternative labels where YAML and JSON both
define one, `issued`'s format MUST having no stated consequence for non-conformance where
`status` and `visibility` both get an explicit safety valve, and the body's
CommonMark compatibility being stated only in the media-type registration rather than in
the section that defines extraction. **Resolved:** all eleven, in #28. The body is now
normatively CommonMark in the section that defines extraction, section 6.2.3 states the
alternative-labels limitation and requires the labels be kept as text, and every format
requirement carries the same preserve-rather-than-reject valve.

Implementing composition surfaced five more ambiguities in section 5.1, the section whose
own claim is that two readers would agree. The one with teeth is that dropping empty
values is in practice a third narrowing rule 6 never enumerates: a literal reading of
"concatenates the values of all composed Frames" gives three guidance entries where the
committed fixture has two. **Resolved:** rule 6 now says an empty value is meaningful and
distinct from no value. It contributes nothing when values concatenate, since there is
nothing to add, and it is present for replacement, which is how a Frame clears a value it
would otherwise inherit. `expected-model.json` matches the current implementation byte for
byte, verified today.

### Excluded from this task's definition of done

| Item | Why excluded |
|---|---|
| Nebari Frames imports all 18 spec examples (the 0-of-18 to 18-of-18 flip) | Requires the parser and `Slots` rewrite in this repo. Real, and the reason for the work, but it is the implementation task that follows. Claiming it here would let a written document take credit for code that does not exist. |
| Collab round-trips a ten-refinement Frame without losing content | Not our repository. The spec can make it possible; it cannot make it true. |

## 11. Decisions

| | Decision | Alternative rejected | Rationale |
|---|---|---|---|
| D1 | Multi-standard crosswalk | Single parent standard (schema.org profile, or Dublin Core application profile) | No single standard covers identity, versioning, lineage and content typing. schema.org would make JSON-LD canonical and Markdown second-class, fighting both PR #25 and the AGENTS.md-shaped reality. Dublin Core has no vocabulary for a resource's content, effectively no versioning and no lineage, so it would need heavy extension anyway. Crosswalking is what DCAT, ADMS and RO-Crate did. |
| D2 | Ten content elements return as optional refinements with a normative dumb-down rule | Authoring template only; or flat peer elements | Template-only leaves structured content unexchangeable, so section-level features have no portable representation. Flat peers lose graceful degradation: a reader ignoring `rules` would drop the content with no rule saying where it should go. Refinements plus dumb-down satisfy PR #25's minimalism and our typed content simultaneously. |
| D3 | Obligation floor is `identifier` and `guidance` only | Mirror v0.2's four required fields; or DC-strict with nothing required | RFC 2119 section 6's test admits only what interoperation requires. Mirroring v0.2 would carry `type` (a serialization artifact) and `visibility` (undefined) into the model. DC-strict would leave the shared spec unable to state any interoperability floor. |
| D4 | v0.2's text is preserved as the Markdown binding | Amend v0.2 in place | Preserves PR #25's work verbatim, makes the bidirectional compatibility claim provable, and makes the proposal additive rather than a reversal. |
| D5 | Composition declared Frame-native | Map to `schema:isBasedOn` or `prov:wasDerivedFrom` | Both mean derivation. Section 3.3's DCAP rule forbids contradicting a borrowed term's meaning. Honesty about the one genuine gap is more credible than a forced mapping. |
| D6 | PyYAML is an optional dependency with a degraded fallback | Hard dependency; or stdlib-only as `validate_frames.py` is today | A YAML encoding cannot be honestly validated without a YAML parser. But the spec repo's zero-install property is worth keeping for casual use, and the line-based fallback already exists in the current tool. So: full validation with PyYAML present, current-tool behavior plus a warning without it. Strictly no worse than today in either mode. |
| D7 | Profile is driven from `frame-core.csv` at runtime | Hard-code the element set in the validator | Makes prose-versus-profile drift detectable (journey 3) rather than latent, and lets an implementation declare its own profile against the same template. |
| D8 | Identity changes MUST be recorded in `derivedFrom` | Leave identifier handling to implementations | One sentence and one existing element convert every silent fork into a visible one. Without it, importing another organization's Frame quietly claims a new identity for it. |
| D9 | `canonicalSource` defined as MAY, not emitted by Nebari Frames yet | Omit until a second registry exists | Identifiers unique only within one registry are ambiguous the moment a second exists, and retrofitting identity is the "cheap now, migration later" failure strategy goal 1 warns about. Defining the element costs nothing; emitting it before there is anything to disambiguate against would be noise. |
| D10 | All refinements are repeatable at the model layer; profiles narrow | Make some refinements (e.g. `style`) non-repeatable in the model | Concatenation never loses content, so it is the safe default. Narrowing to non-repeatable is exactly the technical constraint DCAP permits a profile to add. Our profile narrows six of the ten, which is what the current implementation already does. |
| D11 | Reference grammar is permissive: any string is a `<name>` | Strict grammar the validator enforces | A strict grammar would reject valid v0.2 Frames whose `inherits` values are arbitrary strings. The grammar exists so readers can declare what they resolve, not so validators can reject. |
| D12 | `guards` is a metadata-level relation that composes by accumulation | Defer to a future version (the original N4); or make it an eleventh refinement | Both source documents require Guard references now, so deferral was a misalignment. As a refinement it would dumb-down into prose fed to the model, which is the wrong failure mode for a validation requirement. Accumulation follows the whitepaper's compliance example: a parent's Guard must not be droppable by a child's silence. |
| D13 | The spec is one document in Internet-Draft structure, written as plain GitHub-flavored Markdown | Five Singapore-Framework files (the original plan); kramdown-rfc source rendered to xml2rfc text and HTML (the first pass) | The RFC checklist forced sections the five-file plan lacked: Security Considerations, IANA registries, media types, a formal reference grammar. kramdown-rfc rendered cleanly, but its source does not render on GitHub, which is where frame-spec is read. Plain Markdown keeps every RFC section and stays reviewable; the kramdown source is kept for eventual submission. |
| D14 | Registered `status` and `visibility` values are SHOULD; unregistered values MUST be preserved | Closed MUST vocabularies (the first draft) | Nine of frame-spec's 18 examples carry `status: stable`, and v0.2 lists visibility values as suggested. A closed MUST set made this design's own compatibility claim false for the stewards' examples. SHOULD plus preservation keeps the claim true, matches v0.2's wording, and leaves the registries in place. |

## 12. Open questions

1. **Where does the spec live?** [frame-spec#27](https://github.com/openteams-ai/frame-spec/issues/27) asks whether it moves to nebari-dev. A model-plus-bindings structure is easier to co-own than a file format one party's parser defines, so the answer matters more now than it did.
2. **Does `scope` deserve a better mapping than `dcterms:audience`?** The fit is partial. `dcterms:coverage` includes "the jurisdiction under which the resource is relevant", which is closer in spirit but spatial and temporal in DC's intent. Currently flagged partial rather than resolved.
3. **Term URIs.** A crosswalk needs somewhere to publish Frame-native terms (`guidance`, `composition`, the ten refinements). That is a governance decision tied to question 1.
4. **Registry contract.** The second half of the strategy's Frame protocol (N1): grants, revocation, effective permissions, and enforcement of the ownership and visibility this spec only declares. Not yet scoped. Goal 1 is not met without it, and the implementation task that follows this spec should not be mistaken for it.

Resolved during review: identifier minting and identity preservation (section 6.5, D8, D9). It decomposed into three parts with different stakes: the minting policy is a profile concern and Nebari Frames' is stated; identity preservation on change is a model rule; cross-registry disambiguation is an optional element defined now and emitted later.

## 13. Relationship to the OpenTeams open-source product strategy

Strategy goal 1 requires "one public Frame protocol" representing "accountable ownership, inheritance and precedence, attribution, grants and revocation, effective permissions, and stable Guard references".

| Requirement | Lands in |
|---|---|
| Inheritance and precedence | This spec, section 7, plus conformance profiles |
| Canonical identity | This spec, `identifier`, plus the registry contract |
| Attribution | This spec, `maintainer`, `derivedFrom` |
| Accountable ownership | Declared in this spec (`maintainer` as the accountable party); enforced by the registry contract (N1). The same declared-versus-enforced split the design already uses for `visibility`. |
| Grants and revocation | Registry contract (N1) |
| Effective-permission explanation | Registry contract (N1), [nebari-frames#64](https://github.com/nebari-dev/nebari-frames/issues/64) |
| Stable Guard references | This spec, `guards` (reference only; resolution belongs to the Guard contract) |

Five of the seven rows touch the Frame model; the two that do not are the registry's. What the strategy calls "one public Frame protocol" is therefore two documents: this spec, which settles the artifact, and a registry contract, which settles who may do what to it. Both are required for goal 1. Attempting to answer the registry half inside `spec/v0.2.md` is what has made the whole thing look intractable, and this design's contribution is to separate the halves so each can be finished; it does not claim the second half is done.

Two consumers outside the spec depend on specific parts of it, and the spec should name them as exports so the dependency is visible:

- **The Cog and Op contracts.** The strategy's Cog contract "declares ... Frame dependencies," and the whitepaper has Cogs encapsulate "one or more Frames" and Ops apply "additional Frames at the workflow level." Revision 9's section 5.6 makes this concrete: the example Op manifest carries a `frames:` block listing `required-frame-guard` and `frames_used`. Rule 9's reference grammar is the form those entries take. The `frame-spec` gap analysis asked for "the minimum interoperable contract those systems rely on"; the grammar plus `identifier` and `version` is that contract.
- **Tracks.** A Track records "the Frames applied," and pre-flight Guards check "whether the right Frames were applied." Both need to name a Frame unambiguously. `identifier` and `version` are what a Track records; nothing else in the model is required for that purpose. An earlier revision of this design gave the Representation a `checksum`, and #28 cut it: it describes a stored copy rather than the Frame, no encoding carried it, and a Track that needs to pin bytes can hash them itself.
