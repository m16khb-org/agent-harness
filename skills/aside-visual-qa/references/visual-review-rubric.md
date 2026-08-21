# Visual Review Rubric

Use this rubric after exercising the real user journey. A checkbox without
browser evidence is not coverage.

## Authority order

1. User-provided acceptance criteria and design reference.
2. Repository requirements and design-system documentation.
3. Existing product patterns.
4. Normative WCAG 2.2 requirements.
5. Recognized usability guidance.
6. Reviewer preference, explicitly labeled as preference.

Primary references:

- [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- [WAI-ARIA APG](https://www.w3.org/WAI/ARIA/apg/patterns/)
- [Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html)
- [Keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html)
- [Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html)
- [Focus Not Obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html)
- [Target Size (Minimum), 2.5.8](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)
- [Dragging Movements, 2.5.7](https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements.html)
- [Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html)
- [Ten Usability Heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/)
- [NN/g Error-Message Guidelines](https://www.nngroup.com/articles/error-message-guidelines/)

## Review matrix

| Category | Browser-observable checks |
|---|---|
| Intent | Primary task and call to action are obvious; content and order match the stated purpose |
| Hierarchy | Headings, grouping, emphasis, and reading order communicate importance |
| Layout | Alignment, spacing, density, containment, and scroll ownership are coherent |
| Typography | Readable size/line length/line height; no clipping, orphaned CJK fragments, or metric mismatch |
| Color | Text and controls remain perceivable; states are not color-only; body/UI text contrast ≥ 4.5:1 (large text ≥ 3:1) |
| Components | Same function uses consistent anatomy, naming, icons, focus, and states |
| Targets | Pointer targets are ≥ 24×24 CSS px (2.5.8) or equivalently spaced; drag interactions have a non-drag alternative (2.5.7) |
| Reflow | Content remains usable at required widths/zoom; no accidental two-axis scrolling or overlap |
| Keyboard | Core flow works with keyboard; no trap; order is logical |
| Focus | Focus indicator is visible and not hidden by sticky or modal UI |
| Accessibility tree | Controls expose useful names, roles, values, and changing states |
| Forms | Labels, required state, format help, errors, suggestions, and preserved input are usable |
| System states | Loading, empty, error, success, disabled, and retry states communicate status and next action; a failed action never leaves a prior success message on screen |
| Feedback timing | Interaction-to-feedback stays within flow-preserving bounds (~400ms Doherty threshold); slower actions show progress or skeleton feedback |
| User control | Cancel, undo, close, escape, and recovery exist where the task requires them |
| Motion | Motion has purpose; reduced motion preserves meaning and task completion |
| Copy | Labels and instructions are clear, concise, consistent, and not visually awkward |

## Severity

- `Blocker`: core task cannot be completed, a trap exists, or a severe
  accessibility barrier prevents use.
- `High`: major task friction or trust/accessibility failure with no practical
  workaround.
- `Medium`: meaningful degradation with a workaround or narrower affected
  population.
- `Low`: minor friction, inconsistency, or polish issue with limited impact.

WCAG conformance level and defect severity are different fields. Do not call a
finding `High` merely because it references Level A or AA.

## Finding quality gate

A valid finding answers all of these:

1. What did the user try to do?
2. What should have happened, and which authority says so?
3. What was observed in the current browser state?
4. How can another person reproduce it?
5. Who is affected and how?
6. What evidence supports the statement?
7. What concrete outcome should change?
