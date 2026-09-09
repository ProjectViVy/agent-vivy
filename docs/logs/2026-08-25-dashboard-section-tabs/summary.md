# Dashboard sections match Settings grouping

Date: 2026-08-25
Status: complete

## What changed

Dashboard (`/dashboard`) now uses the same tab + card grouping as Settings:

- Tabs: Overview / Token / Audit
- Overview: one Runtime Status card (Sessions, Active Runs, Pending Reviews) and one Recent Activity card
- Token / Audit: one card each, with title and description

Removed the three separate KPI cards and the single long scroll of every block.

## Unchanged

Token fake data, AuditPanel contents, DemoBanner on the dashboard route.

## Scope

UI only.
