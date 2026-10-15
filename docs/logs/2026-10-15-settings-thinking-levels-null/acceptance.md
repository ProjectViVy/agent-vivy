# Acceptance: settings no longer takes the frame down (2026-10-15)

## Human check

1. Start the split pair (`just dev`) and open `http://127.0.0.1:3015`.
2. Open **设置 / Settings** with an active model that declares no thinking
   levels (the reported setup: provider `openai`, model `abab6.5-chat`).
3. The 思考级别 card shows `The active model does not declare extended-thinking
   support.` / `当前模型未声明扩展思考支持。`

Before this fix the same page replaced the sidebar and the whole route slot
with a `Vivy UI Module failed` panel reading
`UI root component failed: Cannot read properties of null (reading 'length')`.

## What must stay true

- With a thinking-capable model, the card still lists the model's declared
  levels plus `auto` / `on` / `off`, and shows the effective level.
- `model/thinking/levels` answers `"levels": []` — an empty JSON array — when
  the active model declares nothing; it must never answer `null`.
- A malformed reply from any *other* backend list field is still a defect to
  report, but it must not unmount the app frame from this card.
