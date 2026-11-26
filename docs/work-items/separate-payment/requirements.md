# Requirements

| ID | Acceptance |
| --- | --- |
| P01 | Build the existing storefront and management UI with React, Next.js App Router and shadcn/ui. Production Docker serves these and new routes directly. |
| P02 | Preserve provided product names as literal text through import, search, administration and order views. Preserve all records and database behavior when names contain browser markup or SQL-looking punctuation. Keep script execution restricted. |
| P03 | Starting checkout creates a durable pending order and opens a separate simulated provider page, normally in a new tab. Approving or declining on that page initiates processing. UUIDs stay out of visible checkout, payment and history content. |
| P04 | An Orders screen lists persisted orders with recognizable references, timestamps and statuses. It can resume a pending payment and reflects completed and rejected outcomes. Existing orders remain visible after migration. |
| P05 | Preserve idempotency, stock validation, authoritative pricing, compensation and availability during broker interruption. Duplicate payment submissions must resolve to the original choice and reject a conflicting outcome. |
| P06 | Update the English runbook, architecture, prompt history and verification skills. Exercise the live UI, exact fixture, two-step checkout, asynchronous compensation, and isolated Docker cleanup. |
