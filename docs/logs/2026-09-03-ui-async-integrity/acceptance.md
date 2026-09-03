# Acceptance

- An older marketplace response cannot overwrite a newer query; operations on different skills retain independent busy/error state.
- Switching sessions while compaction history loads cannot commit the old session response; refresh errors preserve visible records.
- Saving provider settings cannot overwrite an unsaved HTTP draft, and only the submitted section reports saving.
