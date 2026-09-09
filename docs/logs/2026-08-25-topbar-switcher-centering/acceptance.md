# Acceptance method

1. Start the split development loop with `just dev`, then open http://127.0.0.1:3015
2. Observe the top bar: navigation button + Vivy avatar/status on the left, and
   session and to-do icons on the right
3. The centered “Mask | Model” pill switcher should sit exactly in the middle of
   the top bar, with visually symmetric spacing on both sides
4. Drag the browser window to change its width (≥768px): the switcher remains
   centered; as the window narrows, pill text contracts with an ellipsis without
   overlapping the side content; below 768px the switcher is hidden as designed
5. Click the switcher: the “Select Mask” and “Select Model” dropdowns remain functional
