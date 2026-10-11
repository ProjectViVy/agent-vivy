# Native plugin format and dedicated UI

Revised [the bundle architecture](../../architecture/VIVY-CONTENT-BUNDLES.md) following the user's correction: “不用做转换，需要定义vivy原生插件格式。还有一件事，需要有专门的ui板块”.

The design now accepts only the proposed Vivy-native content plugin format. Removed the authoring converter, manifest detection, compatibility pipeline, conversion report, and associated delivery tasks. Codex/ZCode remain research references. Role-versus-procedure classification remains a native authoring principle.

Added a required first-level Plugins section to Vivy with library, native installation, details for all five component kinds, lifecycle controls, update preview, consent/bindings, operation blockers and recovery. Cross-links preserve the existing Mask/Skill owners. A basic UI accompanies the first usable slice; complete plugin management is required for v1 acceptance.

This is a design correction. No implementation plan, runtime/UI code, dependency, schema migration, Git commit or external publication was made. No release notes are needed.

## Naming correction r2

The user subsequently specified “不能叫做插件。而是叫做工具包。” The current architecture and UI contract now use Toolkit / Toolkits (工具包). Bundle remains the technical content packaging term; existing compiled and upstream plugins keep their original names. Earlier text above records the previous revision. This correction changes documentation terminology only.
