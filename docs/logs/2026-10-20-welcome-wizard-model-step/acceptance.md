# Acceptance — welcome wizard model step

The first-run wizard must let a person pick a provider, pick that provider's
protocol endpoint, pick a model, and enter an API key without knowing any
internal identifier.

## Human check (split pair)

1. From the lane root run `just dev` and open `http://127.0.0.1:3016` in a
   browser profile with empty `localStorage` (or reset
   `vivy.ui.welcome.completed`).
2. The welcome wizard opens on the intro step. Click **Next**.
3. The model step shows three selects and no free-text protocol/Base-URL
   fields:
   - provider select lists catalog vendors (on the pruned catalog:
     DeepSeek / SenseNova / MiniMax);
   - protocol select lists only that vendor's executable endpoint variants;
   - model select lists the selected endpoint's models;
   - the chosen endpoint address is shown below them.
4. With no stored key, **Next** is refused with the key-required message. The
   "runtime environment already provides the key" checkbox appears.
5. Type a fake key (`sk-test`): the checkbox disappears, **Next** saves and
   moves to the done step. Re-open the wizard from Settings → General: the
   provider/protocol/model are pre-filled from the saved selection and the key
   field is empty with a "configured" hint.
6. Alternatively, check the env-injection box and click **Next**: the wizard
   saves the selection without writing a key.
7. Pick **Model settings** on the done card: Settings → Model opens with the
   same endpoint row marked as selected.

## Read-only / frozen

Point the UI at a deployment whose settings are read-only or whose provider is
frozen by environment variables. The model step shows the corresponding notice,
keeps every control disabled, and **Next** advances without writing settings.

## Regression checks

- No "Open in browser" button and no DeepSeek platform link remain in the
  wizard.
- `openai-responses` is not in this generation's sealed protocol set (the
  provider-agentic migration removed it), so it cannot appear as an option.
- A previously configured custom gateway is not silently replaced when the
  wizard is re-run (the model step starts with empty selects).
- Switching provider clears the protocol/model/key draft so a key cannot be
  posted against the wrong endpoint.
