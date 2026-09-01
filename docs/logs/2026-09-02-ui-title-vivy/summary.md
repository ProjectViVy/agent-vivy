# UI 标题定名 VIVY

## What changed

- `ui/index.html`：静态 `<title>Agent Diva 前端演示</title>` → `<title>VIVY</title>`（2026-09-02 拍板定名）。
- `ui/src/i18n/en.ts` + `ui/src/i18n/zh.ts`：`app.documentTitle` `Vivy` → `VIVY`——`__root.tsx:20` 挂载后动态覆盖 `document.title`，静态值与运行时值统一，中英一致。
- 新增 `ui/e2e/app-title.spec.ts`：真实 3015 路径断言 `page.toHaveTitle('VIVY')`（既有 spec 同款 welcome-skip 引导），把标题变成常驻回归。

## Explicitly not done

- `app.loading`/`app.cannotConnect` 等 i18n 文案里的 "Vivy" 称谓不在本行（那是产品称谓不是标题，拍板范围是「UI 标题」）。
- 顶栏 wordmark、Studio 皮肤（submodule）未动。

## Notes

- 加载闪屏 wordmark 本就渲染 `VIVY`（`__root.tsx`），本次只对齐了浏览器 tab 两处来源。
