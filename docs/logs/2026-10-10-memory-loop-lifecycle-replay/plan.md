# Scope and implementation choice

Execute independent existing S04/S09 control/storage checks while S08 recall wiring remains local work. Use the supported public control actions for mutation/search/expansion/receipt, the owned native capture sink for conflict, and the actual host cursor/journal path for redelivery. Rewind only a task-owned Snapshot through its public CAS interface; never reset runtime windows or user profiles.

The design adds no model/runtime/authority path. Existing Eino v0.9.13 Service.Run and native schema.Message mapping are unchanged; no custom orchestration capability is introduced. Cost: two integration tests plus handoff records, no dependencies or product configuration. Full Story evidence still requires its original prerequisites and frozen candidate.
