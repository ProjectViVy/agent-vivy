# R1 rollback

Revert this story's commit. Migrations 039/040 are additive columns/tables —
rolling back code leaves inert schema (`cron_jobs.revision`,
`report_generations`, `notebook_entries_series_uniq`); a full teardown
requires reversing those migrations on live databases. The default recipe
loses `vivy/reports` and the four `vivy.reports.*` actions disappear with
it; `recipes/reports-backend.vivy.yml` (R0) also stops building a report
capability. No data migration needed for generated reports: deleting the
module leaves previously published report entries as ordinary notebook
entries.
