# Acceptance

## Operator view (DeepSeek `deepseek-chat`)

1. Select `deepseek-chat` (settings document or `VIVY_MODEL=deepseek-chat`).
2. Ask Vivy to delegate a task that reads a PNG (`shot.png`) in the workspace.
3. The child run completes. Its tool result says the file is an image and the
   active model does not accept image input, instead of the provider banner
   “模型服务出错 / 无法连接！请检查供应商配置！”
4. `gateway.out.log` has no `child run failed ... status code: 400 ... (2013)`
   entry, and no later request in that run carries an image.

## Attachment boundary

- A UI/RPC image attachment on `deepseek-chat` is rejected before the turn with
  `model "deepseek-chat" does not support image attachments`.
- Inbound channel images are dropped with a warning and the text turn still
  runs.
- A custom gateway model whose `supports_images` is absent keeps the old
  permissive behavior (unknown, never guessed).

## Vision routes unchanged

A route that declares `supports_images: true` (`deepseek-flash`,
`deepseek-v4-flash`) still receives the image and answers from it; the existing
`TestReadFileReturnsImagePartsEnvelope` and the app E2E with the default model
keep passing.
