# Acceptance

- A custom provider can have the same name as an embedded vendor without replacing that vendor's environment credential. The custom endpoint continues to resolve its own stored credential. Built-in settings overlays still use their declared credential variable.
- Several environment credentials do not silently pick catalog order. Select a vendor with `VIVY_PROVIDER` or `config.providers.active`; an adapter override resolves only when unique, otherwise configured selection may disambiguate. If no candidate can be chosen, settings remain editable/effective and startup warns without showing credential values.
- A non-empty provider default missing from its model list appears after loading an older document or saving the provider, can be selected, and is not duplicated by repeated loads or saves.
- Refresh accepts 2,000 valid unique models, skips IDs longer than 256 bytes after trimming, and rejects a 2,001st accepted ID with an explicit error. Failed refresh leaves the registry and active selection unchanged.

The browser checks were completed on the owner-approved split URL `http://127.0.0.1:3016`, with backend `8788`, disposable data, and a local mock. See `browser-smoke.png` and `verification.md` for observed behavior and validation limits.
