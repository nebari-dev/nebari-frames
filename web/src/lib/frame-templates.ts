// Starter templates for new Frames. A template is nothing more than a
// pre-filled body (and a hint of the metadata that usually accompanies it):
// Frame Spec v0.2 defines no body structure, so templates are editorial
// starting points, not schema.

export interface FrameTemplate {
  id: string;
  label: string;
  /** Shown under the picker so the author knows what they are starting from. */
  hint: string;
  /** Suggested `scope` metadata; applied only when the field is still empty. */
  scope?: string;
  body: string;
}

export const FRAME_TEMPLATES: FrameTemplate[] = [
  {
    id: "team-norms",
    label: "Team norms",
    hint: "How a team works together: expectations, communication, decisions.",
    scope: "team",
    body: `Describe how this team actually works, so an AI assistant behaves like a member of it rather than a stranger.

Decisions and their reasoning are written down where the team can find them; a decision that lives only in a call did not happen.

Prefer asynchronous communication. Interrupt someone only when the work is blocked.

When two conventions conflict, the one written here wins; update this Frame when the team changes its mind.`,
  },
  {
    id: "code-review-norms",
    label: "Code review norms",
    hint: "What reviewers block on, how to label comments, when to approve.",
    scope: "team",
    body: `Block on correctness, security, and data loss. Everything else is a suggestion the author is free to decline.

Say which one you mean. Prefix blocking comments with "Blocking:" and everything else with "Nit:" so the author can triage a long review at a glance.

Review the change that was made, not the change you would have made. If a different approach is genuinely better, say so once, explain why, and leave the decision with the author.

Approve when the change is safe to merge, not when it is perfect. A follow-up issue costs less than a stalled pull request.`,
  },
  {
    id: "brand-voice",
    label: "Brand voice",
    hint: "Voice, tone, and messaging guardrails for outward-facing writing.",
    scope: "company",
    body: `Plain, direct, technically credible. Short sentences. Write for a busy expert reader.

Never claim performance numbers without a citation. Avoid superlatives ("revolutionary", "best-in-class") in customer-facing copy.

Key terms and how we use them:

- **customer**: an organization, not an individual user.

When uncertain whether a claim is approved messaging, flag it for review rather than asserting it.`,
  },
  {
    id: "project-context",
    label: "Project context",
    hint: "Orient work on one project: what it is, architecture, conventions.",
    scope: "project",
    body: `What this project is, in two sentences, and who it is for.

## Architecture

The pieces of the system, how they talk to each other, and where the code for each lives.

## Conventions

The rules that would surprise a newcomer: naming, testing expectations, how changes ship.

## Out of scope

What this project deliberately does not do, so effort is not spent re-litigating it.`,
  },
  {
    id: "business-process",
    label: "Business process",
    hint: "A procedure work must follow: steps, owners, and hand-offs.",
    scope: "department",
    body: `Describe the process end to end: what starts it, the steps in order, who owns each step, and what "done" means.

1. Intake — where requests arrive and what a complete request contains.
2. Review — who approves, against what criteria, and the expected turnaround.
3. Execution — the work itself and any constraints on how it is performed.
4. Hand-off — what is delivered, to whom, and where it is recorded.

Note the exceptions explicitly: what may be skipped, by whom, and what must never be skipped.`,
  },
];
