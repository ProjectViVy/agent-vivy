# Verification

The prior runtime suite failed `TestProjectInstructions_SamePathOnce` and `TestProjectInstructions_RunCarriesSnapshot` with the managed ancestor. Focused rerun and the affected 1048-pass regression are green after fixture root isolation.

Required `just ci` reached `TestExternalModuleConsumesPublicAPI` and failed obtaining VCS status from the managed ancestor despite the parent environment, because the fixture overrides GOFLAGS. Explicit synthetic-build `-buildvcs=false` fixes that failure; targeted SDK host regression observed GREEN. Full CI rerun is pending the final checkpoint.
