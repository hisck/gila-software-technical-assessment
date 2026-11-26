# Requirements

| ID | Required behavior and acceptance |
| --- | --- |
| E01 | Create, list, read, edit and delete products from the UI and API. Support the seven CSV fields. Enforce unique active SKUs, validate quantities and exact decimals, and reject stale edits. |
| E02 | Import the supplied CSV in the UI and API. Normalize USD currency markers and `free`. Skip blank rows, report invalid or duplicate rows with their starting source line, and preserve existing inventory on reimport. A malformed CSV aborts before writes. |
| E03 | Search name, SKU, description and category in the storefront. Filter by category and page results. Cache recently used queries and ensure catalog events refresh the visible projection. Search reads remain available when the catalog or broker is offline. |
| E04 | Buy one or more products with a simulated approval or decline. Show pending and terminal status in the UI. Use catalog prices and reject stale quotes. Reserve stock atomically across all lines. Restore stock on a declined payment before reporting the final decline. |
| E05 | Prevent duplicate purchases from network retries using a client-generated idempotency key and a durable uniqueness constraint. Identical requests return the same order ID. Reusing a key with another payload returns a conflict. Concurrent orders cannot oversell. |
| E06 | Run catalog, search, orders and payments as separate Docker services backed by local PostgreSQL. Exchange domain commands and events through RabbitMQ with persistent delivery and consumer deduplication. Redis may fail without preventing purchases or uncached searches. |
| E07 | Provide reproducible Docker instructions, the verified CSV download date, the English prompt translation, and English design decisions including alternatives. Keep program source free of comments as required by the assessment. |
| E08 | Support keyboard operation and mobile layouts. Provide actionable validation and retry feedback without collecting payment or personal data. Keep infrastructure and service endpoints private to the Compose network. |
| E09 | Resume through `.ai-development.json`. Run required Ponytail review and independent QA. Add a discoverable verification skill that launches a dedicated Compose instance, drives the product and checkout paths, keeps evidence, and tears down its own resources. Prove the critical order-state invariants under duplicate and out-of-order events with a focused, repeatable check. |

## Boundaries

- Judgment: USD follows the source CSV's dollar notation. Price accepts up to two decimal places and weight up to three. Zero price, stock or weight is valid.
- Judgment: names and categories are required. Blank weights, missing names or categories, and negative stock produce row errors. Duplicate resolution is first valid row wins with preservation of existing products.
- Backstop: leading or trailing whitespace is trimmed. SKU comparison is case-insensitive for the restricted ASCII SKU alphabet. Import reads UTF-8 and accepts a single leading BOM. A bounded upload protects request and transaction size.
- Backstop: edit and delete include an observed product version. Stock mutations change this version so an administrator cannot overwrite an intervening reservation.
- Backstop: a declared unit price accompanies each cart line. The catalog validates every line inside a single transaction as the final check before charging.
- Judgment: the assessment runs locally with a simulated payment outcome selected in the purchase UI. Public multi-user administration and live payments require a separate scope.
