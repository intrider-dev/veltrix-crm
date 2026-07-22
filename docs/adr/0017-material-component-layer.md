# ADR 0017: One Material-based component layer, not a second UI kit

- Status: accepted
- Date: 2026-07-22

## Context

The product needs consistent buttons, selects, fields, density, focus treatment, errors, and overlays. A second universal kit would duplicate accessibility behavior and tokens, increase payload, and make the interface visually inconsistent.

## Decision

Angular Material 3 remains the only universal UI kit. Product controls use shared design tokens and thin project components/styles over Material primitives. Native controls are reserved for cases where their platform behavior is useful and are wrapped by the same field contract; complex selectors use `mat-select`. Icons are small inline SVG symbols through the shared icon component, not a font or full icon bundle.

All controls must preserve visible focus, keyboard operation, minimum touch targets, localized labels/errors, compact and comfortable density, high contrast, and reduced motion. Feedback that needs no user decision uses the bounded toast stack; blocking or durable failures use an inline/persistent error panel.

## Consequences

- Buttons, select arrows, field padding, and validation states have one maintained implementation surface.
- No second UI framework or global CSS reset is added.
- Feature teams may create small domain components, but cannot fork button/input behavior or hard-code user-visible strings.

