# Tasks

All tasks belong to the Fieldwork Market Git root.

| ID | Depends on | Requirements | Verification |
| --- | --- | --- | --- |
| T01 | — | P01–P06 | Freeze contract and stage scoped parallel work. |
| T02 | T01 | P03–P05 | Orders migration, pending order, payment and history endpoints. Run Go and live Docker checks. |
| T03 | T01 | P01–P04 | Next.js and shadcn/ui migration. Build payment and history routes and verify browser flows. |
| T04 | T01 | P01–P02, P06 | Nginx static export, script hashes, exact fixture regression, maintained runbook and skills. |
| T05 | T02–T04 | P01–P06 | Independent review, real browser and isolated runtime verification, work-item coverage. |
