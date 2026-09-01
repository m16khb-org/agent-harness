# UI source catalog

This catalog describes selection intent, not a frozen API. Official sites can
change component names, commands, versions, licenses, and pricing. Re-open the
linked page before using a source.

## Role matrix

| Source | Best role | Strong fit | Avoid when |
|---|---|---|---|
| [shadcn/ui](https://ui.shadcn.com/docs) | Accessible behavioral foundation | Product controls, forms, overlays, navigation, data display, consistent local ownership | The project already has equivalent primitives or is not in its supported ecosystem |
| [shadcnblocks](https://www.shadcnblocks.com/docs/blocks/getting-started) | Page-level structure | Landing sections, auth, pricing, settings, dashboards, and fast page scaffolding | A block would dictate the product's identity or introduce unnecessary sections |
| [Beautiful UI](https://beautifului.dev/) | Crafted AI-agent states | Thinking, loading, streaming text, approvals, tool activity, task rows, chat, and agent-human communication | The product is not AI-native or only needs a conventional chat primitive |
| [AI SDK Elements](https://ai-sdk.dev/elements/overview) | Functional AI interface composition | Streaming conversations, messages, reasoning, prompt input, attachments, tools, and AI SDK workflows | The implementation does not use the relevant React/AI SDK stack or needs only decorative AI visuals |
| [Transitions.dev](https://transitions.dev/) | Product transition patterns | State continuity, modal/page/card changes, feedback, and transition refinement | A static surface has no meaningful state change |
| [beUI](https://beui.dev/) | Animated component source | React/Next.js components using Motion and Tailwind, distributed through a shadcn registry | The project should not add Motion or already has an equivalent component |
| [Rare UI](https://www.rareui.com/components) | Distinct animated accent | A rare, self-contained interaction or visual moment installed as owned source | Repeating the treatment across ordinary product controls |
| [Magic UI](https://magicui.design/docs) | Marketing and visual effects | Text effects, backgrounds, marquees, borders, reveals, and polished landing-page accents | Behavior-heavy controls, dense application screens, or effects without hierarchy |
| [Aceternity UI](https://ui.aceternity.com/components) | Cinematic hero and showcase interaction | High-impact heroes, cards, backgrounds, and motion-led marketing surfaces | Performance-sensitive dense UI, subdued brands, or multiple competing hero effects |

## Composition recipes

### Conventional product screen

- Existing design system or shadcn/ui for controls.
- Custom composition for product-specific layout.
- Transitions.dev principles for state changes.
- No accent library unless the screen has one justified focal moment.

### Marketing or launch page

- shadcn/ui for interactive controls.
- shadcnblocks for structural starting points.
- Choose one of Magic UI, Aceternity UI, or Rare UI for the dominant visual
  language.
- Use Transitions.dev or the chosen library's own motion, not both on the same
  interaction.

### AI chat product

- shadcn/ui for generic controls and overlays.
- AI SDK Elements for conversation, message, prompt, attachment, tool, or
  reasoning composition.
- Beautiful UI for specialized agent-human states not already covered.
- beUI or Transitions.dev for one consistent motion layer.

### Agent operations dashboard

- shadcn/ui for filters, menus, tables, tabs, dialogs, and feedback.
- Beautiful UI for approvals, tool activity, tasks, thinking, and streaming
  state.
- AI SDK Elements only where a real conversation or generative workflow
  exists.
- One restrained Rare UI, Magic UI, or Aceternity UI treatment for a landing
  or empty state, not the dense dashboard body.

## Source-specific checks

### shadcn/ui

- Inspect `components.json`, style/base choice, aliases, CSS variables, and
  existing primitives before running the CLI.
- Use the official
  [registry documentation](https://ui.shadcn.com/docs/registry) for namespaced
  registries.
- Preserve the underlying accessibility behavior while adapting visuals.

### shadcnblocks

- Confirm whether the selected block and dependencies are available under the
  required access tier.
- Treat blocks as scaffolding. Replace demo content and normalize them to the
  product's tokens and primitives.
- The official getting-started guide currently routes React projects through
  shadcn setup and an `@shadcnblocks` registry; verify the current instructions
  before installation.

### Beautiful UI

- The official site describes copy-paste primitives for AI-native interfaces.
- Select by product state, not appearance: loading, thinking, streaming,
  approval, tool activity, tasks, chat, or input.
- Verify source availability, dependency expectations, and license on the
  current component page before copying.

### AI SDK Elements

- Use the official
  [overview](https://ai-sdk.dev/elements/overview) and component pages to
  confirm the current composition API.
- Keep streaming and tool state driven by real application data. Do not fake a
  reasoning or tool timeline only to fill the design.
- Preserve client/server boundaries and the repository's AI SDK version.

### Transitions.dev

- Start with the official [collection](https://transitions.dev/) and
  [skill page](https://transitions.dev/skill.html).
- Extract the transition principle and adapt it to product tokens; do not copy
  unrelated demo styling.
- Define reduced-motion behavior and verify the current terms before using
  source from free or paid collections.

### beUI

- The official site currently describes copy-paste React/Next.js components
  built with Motion and Tailwind and distributed through shadcn.
- Use the official
  [motion patterns](https://beui.dev/docs/motion-patterns) to justify timing,
  purpose, and reduced-motion behavior.
- Do not add Motion for a decorative component when CSS or an existing runtime
  already covers the interaction.

### Rare UI

- The official component catalog describes single-file components installed
  through the shadcn CLI.
- Verify each component's actual dependencies and source before installation.
- Use rarity as emphasis. Repetition makes a rare treatment ordinary and noisy.

### Magic UI

- Confirm the current component page, install command, dependencies, and
  accessibility behavior in the official docs.
- Prefer effects that reinforce hierarchy or communicate activity.
- Pause or remove continuous animation when it distracts, consumes excessive
  resources, or conflicts with reduced motion.

### Aceternity UI

- The official catalog offers copy-paste React/Next.js components and blocks
  built around Tailwind and Motion.
- Check free/pro access, asset rights, client-component boundaries, and
  animation cost.
- Isolate cinematic effects to a focal section and provide a simpler mobile
  and reduced-motion treatment.

## Provenance ledger

Keep this minimal ledger while implementing:

| Local component | Official source URL | Role | Dependencies | Adaptations | License/access checked |
|---|---|---|---|---|---|

The ledger prevents accidental duplication, invented APIs, lost attribution,
and unexplained dependency growth.
