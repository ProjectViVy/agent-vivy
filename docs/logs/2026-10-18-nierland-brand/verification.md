# Verification — Nierland brand lockup

Everything below was run on the live Studio profile (`:3090`), the submodule
tree, or the profile copy — not on a mock of the skin.

## 1. Skin source and profile copy

The profile install is a pnpm `file:` dependency whose files are **hard links**
to `studio/dsh-vivy-studio/`:

```text
fsutil hardlink list studio\dsh-vivy-studio\theme.css
  \Users\...\morediva\studio\dsh-vivy-studio\theme.css
  \Users\...\morediva\data\studio-home\profiles\vivy-studio\node_modules\.pnpm\
    dsh-vivy-studio@file+...+studio+dsh-vivy-studio\node_modules\dsh-vivy-studio\theme.css
```

Editing with the file tools breaks that link (`linkCount=1`), so each edit pass
was followed by an explicit copy into the `.pnpm` path and a SHA-256 parity
check of `theme.css` / `brand.js` / `index.js`. Final state: in sync
(`theme.css B4F1842F64`, `brand.js A8C8ED1557`, `index.js 5B5D494BE6`).

Static checks before restart:

- `content: "__VIVY_PRODUCT__"` appears for the wordmark, the dialog title and
  the dialog copy (3 declarations, all substituted at boot).
- Both gem data URIs decode to valid XML: `viewBox='0 0 20 20'`,
  fills `#2aa8a4 + #fbf7f0` and `#5fd4cf + #1a1613`.
- `node --check brand.js` exit 0; `import index.js` loads with
  `name=vivy-studio-skin inject=[webServer]`.

## 2. Restart

Two restarts, both through
`.agents/skills/vivy-studio-skin/scripts/restart-studio.ps1` started **detached**
(`Start-Process -WindowStyle Hidden`, never killed from inside the server tree).
`data/studio-home/studio-restart.log`:

```text
listener pid=11864; waiting 12 s for the current turn to settle
taskkill: 成功: 已终止 PID 11864 ...
port free: True
boot process started pid=4964
port up after restart: True
restart-studio.ps1 done
```

Final listener: `node` PID 3984 on `127.0.0.1:3090`, `GET /` -> 200. The boot
process is detached, so Studio no longer dies with the agent session.

## 3. Live server assertions (`GET http://127.0.0.1:3090/`)

| Assertion | Result |
|---|---|
| `<title>Nierland</title>`, no `Vivy Studio` in title | PASS |
| `href="/favicon.svg"` gone; `rel="icon"` is the gem data URI | PASS |
| injected style: `content: "Nierland";` for the wordmark | PASS |
| injected style: no `__VIVY_PRODUCT__` token left | PASS |
| injected style: old `content: "V";` letter tile gone | PASS |
| light and dark gem rules both present | PASS |
| `letter-spacing: 0.22em` + `text-transform: uppercase` | PASS |
| injected script: `window.__VIVY_PRODUCT__="Nierland"` | PASS |
| `brand.js` no longer hardcodes `Vivy Studio` | PASS |
| `--vivy-display` leads with Georgia / Palatino Linotype / Noto Serif SC | PASS |
| dialog title rule resolves to `content: "Nierland";` | PASS |
| dialog copy carries the name twice, wording unchanged | PASS |
| no rendered `content: "..."` string still says `Vivy Studio` | PASS |

Rendered content strings served in the style block: `Nierland`,
`寻找真心之旅`, `STUDIO`, `Nierland`, `Nierland 是独立应用。…你看见的名字是
Nierland。`, `Seeking.....`.

## 4. Pixel check (the mark really paints)

The declarations existing does not prove the data-URI gem paints. The stylesheet
actually served by Studio was loaded into a page with the real
`button:has(svg[viewBox="0 0 182 24"])` structure, rendered by headless Chrome
(`--headless=new`), and analysed with the bundled Python + Pillow (colour masks
plus bounding boxes):

```text
[light.png] gem mark: 76 px  bbox 15x15   wordmark: 97 px  bbox 83x10
[dark.png]  gem mark: 88 px  bbox 15x15   wordmark: 155 px bbox 84x10
OVERALL: PASS
```

Deliberately not asserted: the inner facet colour (it is the ambient sidebar
colour by design in the dark variant) and letter-run clustering (run counts
flip between 8 and 12 with the antialiasing threshold; the uppercase claim is
covered by the CSS assertion).

## 5. Not verified / limitations

- No headless render of the live Studio page: Chrome never finished loading the
  SPA (long-lived connections), so the pixel check uses the served stylesheet in
  a synthetic page. A human hard-refresh of `:3090` is the remaining visual gate.
- The agent model used for this change declares no image input, so the renders
  were not eyeballed by the agent; their geometry was measured instead.
- `just ci` was not run: no Go, UI, or schema file changed. The touched files
  live in the submodule and are not covered by `just ci`.
