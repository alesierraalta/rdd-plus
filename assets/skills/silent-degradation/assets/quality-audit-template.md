# Quality audit — <system / pipeline>

## Outcome

- What it must produce: <...>
- Metric that proves it: <...>  Threshold: <...>
- Goldset: <path> (<n> cases, version/date)

## Stage accounting

| Stage | in | out | dropped | reason per drop | justified? |
|---|---|---|---|---|---|

## Metric validation (deliberate breaks)

| Break applied | Metric before | Metric after | Metric noticed? |
|---|---|---|---|

## Baseline

| Metric | Value | Goldset version | Date | Threshold |
|---|---|---|---|---|

## Loss attribution

| Stage | Ablated (metric delta) | Perfect-oracle ceiling (metric delta) |
|---|---|---|

## Filters / permissions diff

| Restriction | Items excluded | Justified | Unjustified (defects) |
|---|---|---|---|

## Findings

| # | Type (pérdida de calidad / ceguera del sistema) | path:line | Evidence | Fix + the counter left behind |
|---|---|---|---|---|

## Unmeasured

<what nothing observes, and what it would take to observe it>
