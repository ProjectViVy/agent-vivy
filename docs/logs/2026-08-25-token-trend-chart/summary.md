# Token trend chart shape and color

Date: 2026-08-25
Status: complete

## What changed

The dashboard Token Stats “Usage Trend” bars now use Vivy `primary` instead of
`chart-1`/`chart-2` (orange/teal in light mode), sit on a muted track, and
keep a column silhouette:

- Input = `bg-primary`, output = `bg-primary/40`
- Taller plot (`h-40`), `rounded-t-lg`, `gap-2`
- ≤4 buckets: `max-w-20` centered so 3-day does not become three slabs
- More buckets: bars flex to fill the track

## Unchanged

Period data, totals, model table, session table, export, and detail view.

## Scope

UI only. Fake token snapshots unchanged.
