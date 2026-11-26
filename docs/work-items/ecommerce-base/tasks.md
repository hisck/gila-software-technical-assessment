# Implementation tasks

Every task belongs to the `gila-software-technical-assesment` Git root.

| Task | Depends on | Requirements | Implementation and check |
| --- | --- | --- | --- |
| T01 | — | E01–E06 | Freeze HTTP and event contracts. Implement SQL migrations, messaging, cache and error handling. Build all commands and check broker topology. |
| T02 | T01 | E01–E04, E06 | Implement Catalog CRUD, CSV import, inventory commands, Search projection and cache. Verify fixture outcome, stale writes and projection updates. |
| T03 | T01 | E04–E06 | Implement Orders saga, durable idempotency and simulated Payments. Verify concurrent requests, payment decision and compensation. |
| T04 | T01 | E01–E05, E08 | Implement responsive storefront, catalog management and checkout UI against frozen API. Verify in a real browser. |
| T05 | T02–T04 | E01–E08 | Run the Compose topology and end-to-end assertions, obtain independent QA and review, and verify the runbook and prompt translation. |
| T06 | T05 | E09 | Reconcile local configuration, produce and run the discoverable verification skills, check a deliberate fault in an isolated checkout, and obtain the required Ponytail review. Preserve the user's delivery and gate settings. |
