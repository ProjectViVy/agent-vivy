# Acceptance — workflow inputs (W1)

Product view: a workflow author's node bindings now live where the engine reads
them, so saving and publishing produce a runnable definition instead of a
silently ignored edit or a rejected node.

## Manual check

1. Start the split pair (`just dev`, or the workflow suite's own pair) and open
   `/workflows` (sidebar “Workflows”).
2. Type a workflow id and press **Open**; a starter definition is created.
3. Click the node on the canvas. The properties panel's **Inputs (JSON)** field
   must start at `{}`.
4. Enter:
   ```json
   {
     "literal_text": { "literal": "hello" },
     "root_text": { "source": "input", "pointer": "/topic" },
     "upstream": { "source": "a", "pointer": "/result" }
   }
   ```
   Click outside the field; no error appears and the **Unsaved** badge shows.
5. **Save draft** → “Draft saved.” Re-open the draft: the field still shows the
   same JSON, and the task/tools fields are unchanged.
6. **Validate** → “Definition is valid.”; **Publish** → “Published r1 …”.
7. Inspect the published artifact (`inofy.getRevision`): `begin.inputs` holds
   the three bindings and `begin.config` holds only `task` (and `tool_names`
   when set) — never `inputs`.

## Failure paths worth trying

- Enter `[1,2]` or `{"x":{}}`: the panel reports the shape error and the draft
  is not mutated.
- Enter an unsupported binding field (`{"x":{"source":"a","bogus":1}}`): the
  error names the field; nothing is saved.
- Clear the field to `{}` and blur: the node's `inputs` key disappears instead
  of persisting an empty map.

## Out of scope here

Running the published workflow from an idle session stays unavailable until the
W2 delivery lands; that cut also shows the child task receiving the resolved
literal/root/upstream values.
