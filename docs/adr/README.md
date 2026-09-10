# Architecture Decision Records

Each file here records one decision that was hard to reverse, contested, or
surprising, in the form: the context that forced a choice, the choice, its
consequences, and the alternatives that were rejected and why.

An ADR is for reasoning that the code cannot carry. If a comment next to the
constraint would do the job, write the comment instead: a comment is read by
whoever is already looking at the code it explains, while an ADR is for the
reasoning that spans multiple files, or that a single comment would make too
long to sit next to the line it justifies.

Design documents for whole features live in `docs/design/`. Those describe what
was built; these describe why one option was chosen over another. A design
doc can go stale as the feature evolves around it; an ADR does not, because it
is not a description of the current system but a record of a decision made at
a point in time.

Numbered sequentially, `NNNN-kebab-title.md`, never renumbered. An ADR is not
edited once merged except to add a "Superseded by" line pointing at the ADR that
replaced it: the record of what was believed at the time is the point.
