# E-commerce base

- Work item `ecommerce-base` · autonomous workflow · single Git root
- Sources: the user brief, [challenge PDF](../../../Code%20Challenge..pdf), and [original CSV](../../../Code%20Challenge%20E-Commerce.csv).
- Initial state: empty Git history, two reference files, zero remotes, and no project configuration.
- Initial preflight: the repository had no project graph or verification skill. Source discovery used Git and direct file reads.
- Resumed preflight: `.ai-development.json` enables PStack, independent QA, independent review and required Ponytail full. Graphify is disabled. The existing work-item directory matches the configuration. The operational-gate list is empty.
- Download evidence: macOS `kMDItemDownloadedDate` records November 23, 2026 at 22:13:33 BRT.
- Delivery scope: working tree, runnable local stack, and reviewer documentation. GitHub is the assessment's submission format. Repository publication requires an authorized destination.

| Phase | State |
| --- | --- |
| Discovery | Complete |
| Specification | Complete |
| Core implementation | Complete |
| Independent Docker browser QA and code review | Complete |
| Configured verification skills and Ponytail review | Complete |
| Local delivery | Complete |

Implementation judgments include USD, local-only access, a cart with up to 25 lines, first valid CSV row per SKU, and a buyer-controlled simulated payment outcome. The acceptance criteria and consequences are in [requirements](requirements.md) and [design](design.md).

## Verification results

- The [runtime skill](../../../.agents/skills/verify-fieldwork/SKILL.md) ran its doctor and a fresh, eight-container Compose project. All five HTTP smoke groups passed. The fixture imported 87 products, skipped 10 lines, and reported 8 row issues. The cleanup check found no containers, volumes or networks belonging to that verification project. The evidence files remained in its temporary directory.
- Independent browser QA exercised the production storefront and management UI at five widths from 320 to 1440 px. It verified keyboard navigation, CRUD, CSV issues, filtered search, exact checkout totals, decline compensation, and receipt persistence after reload.
- A dedicated Docker recovery check interrupted RabbitMQ, Redis, Catalog and Payments. It observed checkout and catalog events recover, search remain available, Redis fall back to PostgreSQL, and stock return to the original product ID after a SKU was reused. All four dead-letter queues were empty at the final check.
- Go tests passed with the race detector and `go vet` passed. The [checkout invariants skill](../../../.agents/skills/verify-checkout-invariants/SKILL.md) explored 6 reachable states and 30 delivery edges. An isolated mutation produced a counterexample for premature decline finalization. The restored selector passed.
- Independent code review and Ponytail full review passed after repairs. Search skips its unused outbox poller and refreshes cached results with a single Redis call. The shortcut scan found zero markers.
- Configuration preflight discovered both project skills. Work-item coverage passed for the single affected Git root. The tracker and Git providers are configured as `none`, the operational-gate list is empty, and this delivery is local.
