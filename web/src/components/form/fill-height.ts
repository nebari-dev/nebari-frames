/**
 * Classes for a flex-column container whose `Textarea` should fill the height
 * the container was given.
 *
 * `Textarea` renders a `relative` wrapper `div` around the control, so the
 * control is never a direct flex child of the caller's layout. These classes
 * turn that wrapper into the growing flex child (matched by the `div` that
 * contains a `textarea`, so it works wherever the wrapper sits among siblings);
 * the control itself then takes `flex-1`.
 *
 * Deliberately flex rather than `h-full`: a percentage height does not resolve
 * against a parent whose own height comes from `flex-grow`, which silently
 * collapses the control back to its `min-height`.
 */
export const fillTextareaSlot =
  "[&>div:has(>textarea)]:flex [&>div:has(>textarea)]:min-h-0 [&>div:has(>textarea)]:flex-1 [&>div:has(>textarea)]:flex-col";
