# Next.js and separate payment

The user clarified that React, Next.js and shadcn/ui are required. They also requested literal rendering of imported names, a simulated external payment page, and visible order history with pending status and no displayed UUID.

This work item affects the Fieldwork Market Git root. The existing configuration uses Ponytail full, independent QA and review, local verification skills, and local delivery. Graphify is disabled. The starting checkout automatically requested the selected payment outcome after stock reservation.

| Phase | State |
| --- | --- |
| Discovery and specification | Complete |
| Implementation | Complete |
| Independent review and QA | Complete |
| Local delivery | Complete |

## Verification

- Next.js production build exported the storefront, catalog, orders and payment pages. The isolated Docker skill checked all four routes, their build-specific script policies, the 87 valid fixture products and exact text for the two security regression samples. All six smoke groups and cleanup passed.
- Independent Chrome QA verified both fixture names in the storefront, catalog and paid order. It checked the separate provider tab, pending status before payment, approval, resumption, decline compensation and the absence of displayed UUIDs. Four routes passed at 320, 390, 768 and 1440 px.
- The PostgreSQL upgrade preserved all 17 existing QA orders, including seven paid and ten rejected. Each received a unique order number and a timestamp. A later read confirmed numeric ordering across all 19 orders after two browser checkouts. Slashless links preserved the mapped browser port.
- RabbitMQ interruption left the durable order awaiting action or stock confirmation as expected. Restarting the broker completed the already accepted request and reduced stock once. Go race tests and vet passed. The maintained saga model explored seven states and 35 delivery edges, and rejected a deliberate premature transition.
- Independent Ponytail review and its focused follow-up passed. The [verification retries](test-failures.md) record the source and runtime findings. The local configuration leaves delivery integrations disabled.

The simulator runs on the same local origin in a separate presentation. Dates assigned to legacy orders reflect migration time because the previous table recorded no creation timestamp.
