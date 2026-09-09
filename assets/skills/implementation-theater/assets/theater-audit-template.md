# Theater audit — <change / module>

## Production entry points

<route table / CLI / worker / cron / composition root>

## Reachability

| Symbol | Highest rung proven (R0–R5) | Triage verdict | Evidence |
|---|---|---|---|

## No se ejecuta

| # | path:line | Verdict (dead / never wired / flag off / not deployed) | Evidence | Action |
|---|---|---|---|---|

## Se ejecuta pero finge

| # | path:line | Shape (stub / error masking / dead knob / fake machinery) | Proof of execution | Fix |
|---|---|---|---|---|

## Miente sobre sí mismo

| # | path:line | Claim | Line that should make it true | What it does instead |
|---|---|---|---|---|

## Clones

| Copy A | Copy B | Which one runs | Divergence | Fix landed in both? |
|---|---|---|---|---|

## Unproven

<what stayed at R2 or below, and what it would take to prove it>
