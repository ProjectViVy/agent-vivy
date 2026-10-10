# R2 rollback

Revert the story commit `feat(notebook): add report generation and provenance UI`. Safe: changes are additive — new `ReportsClient`/panel/provenance display, one additive i18n block, one new recipe (`no-reports.vivy.yml`), JSON tags on `ReportAdmission` (wire-compatible superset), pin updates. No schema or migration changes; R3 must not build on this commit's UI surface if reverted.
