# Center the top-bar Mask/Model switcher

## Changes

- `ui/src/routes/_layout.tsx`: changed the top bar from `flex + justify-between`
  to a three-column grid `grid grid-cols-[1fr_auto_1fr]`. The centered “Select
  Mask / Select Model” switcher (`MaskAndModelSwitcher`) is placed in the auto
  column with `justify-self-center`, achieving precise horizontal centering
  relative to the entire top bar regardless of unequal side-content widths.
- Added `min-w-0` to the left group and `justify-self-end` to the right group,
  preserving the existing left/right alignment semantics.

## Scope

- Only top-bar layout class names changed; the switcher component and all behavior
  logic were left unchanged.

## Explicitly not done

- The internal structure and dropdowns of `MaskAndModelSwitcher` were not changed.
- The existing mobile breakpoint behavior that hides the switcher below 768px was
  not changed.
