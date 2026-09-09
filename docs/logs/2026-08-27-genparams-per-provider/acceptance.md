# Acceptance — generation-parameter demo moved into 「Settings → General → Advanced Features」

## User view (acceptance steps)

1. Open `http://127.0.0.1:3015/settings`, landing on the 「General」 tab.
2. The 「Model」 tab has only one card, 「Vivy model configuration」—the former standalone
   「Generation parameters」 card is gone; the 「General」 tab shows an 「Advanced Features」
   card (slider-icon card header).
3. The Advanced Features card contains a 「Generation parameters (Demo)」 section + model
   dropdown: the dropdown lists selected models (provider display name · model id) and
   defaults to the current runtime model (or the first model when none is configured).
4. Only the model selected in the dropdown is editable: temperature slider (0.0–2.0 with
   live value) + Max Tokens input; switching models loads that model's own parameters
   (unsaved models return to the 0.7 / 4096 defaults).
5. Click 「Save demo parameters」 and a primary checkmark + 「Saved locally」 appears; after
   refresh the parameters remain (`vivy.demo.gen-params` stores them independently under
   `provider/baseUrl/model`), with the two models not overwriting each other.

## Actual-result criteria

- A second standalone 「Generation parameters」 card appears in the 「Model」 tab = fail.
- The 「General」 tab has no 「Advanced Features」 card / the card has no model dropdown = fail.
- The model dropdown cannot switch, or parameters do not change with the model = fail.
- Saving each of two models overwrites the other = fail.
- The value on the right does not move after dragging the slider or adjusting temperature by
  keyboard = fail.
