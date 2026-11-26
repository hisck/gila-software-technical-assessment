# Fieldwork Market

A local e-commerce reference implementation with product administration, CSV import, search, and simulated checkout. Catalog, Search, Orders, and Payments run as independent Go services and exchange domain messages through RabbitMQ.

## Run locally

You need Docker with the Compose plugin. Run these commands from the repository root.

```sh
docker compose up --build -d --wait
docker compose ps
```

Open [Fieldwork Market](http://localhost:8080). Set `SHOP_PORT` if port `8080` is occupied, for example `SHOP_PORT=8088 docker compose up --build -d --wait`.

1. Open **Manage catalog**. Use **Import CSV** and select `Code Challenge E-Commerce.csv` from the repository. The import form also offers a copy of the example. An empty project imports 87 products, skips 10 lines, and displays 8 row issues. You can create, edit, or delete products from the same screen.
2. Open **Storefront**. Search by name, SKU, description, or category. Filter results or browse by page. Catalog updates appear as Search consumes the events.
3. Add a product to open **Bag** with an item confirmation and subtotal. Choose **Continue shopping** or **Continue to payment**. The order is saved as **Pending payment** and a separate simulator tab opens. Approve or decline there. Open **Orders** to follow the result or resume an unpaid order.

The stack starts with an empty catalog. The sample CSV was downloaded **November 23, 2026** at 22:13 BRT, as recorded by the original file's macOS download metadata. The source file is kept in its original form. [Sample CSV](Code%20Challenge%20E-Commerce.csv)

To review service logs or stop the application, use these commands. Stopping the stack preserves its PostgreSQL and RabbitMQ volumes.

```sh
docker compose logs -f catalog search orders payments
docker compose down
```

## Design decisions

| Area | Decision and alternatives |
| --- | --- |
| Services | Catalog owns products and stock. Search owns a read projection. Orders owns checkout and coordinates compensation. Payments owns simulated decisions. A modular monolith would reduce operational cost. Separate processes meet the requested boundaries and let search continue against its projection while Catalog is unavailable. |
| Storage | PostgreSQL provides unique keys, row locks, exact numeric storage and atomic state-plus-message commits. Each service has its own database and login on one local PostgreSQL instance. SQLite suits a single writer process. A document database would bring different consistency and data-model trade-offs to a workload that already fits relational constraints. |
| Messaging | RabbitMQ provides persistent queues, key-based routing, publisher confirms and consumer acknowledgements. Kafka suits longer retention, replay and many consumers. RabbitMQ fits the command and event flow in one local broker. Browser requests use HTTP. Domain traffic between the four services uses the broker. |
| Checkout | Orders saves an unpaid order before opening the simulator. Selecting an outcome atomically records the choice and queues inventory reservation. Catalog locks products in UUID order, validates price and quantity, and reserves every line in one transaction. Payments records one decision per order. A decline becomes final after Catalog restores stock. Deferring the hold until submission permits a buyer to leave the simulated provider and resume later. |
| Idempotency and cache | PostgreSQL enforces one order per idempotency key. A canonical request fingerprint detects incompatible reuse. Redis accelerates repeated requests through a cached key-to-order mapping validated against persisted data. Search caches normalized queries for 90 seconds of inactivity, keyed by projection revision. A local in-memory cache would duplicate entries across replicas. Both services fall back to PostgreSQL when Redis is unavailable. |
| Search | PostgreSQL provides a paginated, case-insensitive substring search over the event-fed read model. Versioned snapshots preserve freshness through out-of-order delivery, including deletes. A dedicated search engine would add relevance features and indexing infrastructure. |
| Backend | Go offers standard HTTP, bounded concurrency, compiled service binaries, and focused PostgreSQL, AMQP and Redis clients. Java/Spring has mature integrations with a broader runtime and framework surface. PHP can run long-lived workers with additional deployment conventions. |
| Frontend | React, Next.js App Router and shadcn/ui provide the storefront, administration, order history and simulator. Next.js exports each route. Nginx serves the static pages and proxies their API requests. Server rendering remains an option for a future public storefront. [Next.js static exports](https://nextjs.org/docs/app/guides/static-exports) |

The [architecture guide](docs/architecture.md) explains why the services are separate, traces the event-driven flows, and reviews the Go and TypeScript development practices. The [checkout design](docs/work-items/separate-payment/design.md) defines the current API. The [original work item](docs/work-items/ecommerce-base/design.md) records the infrastructure and message decisions. The [English prompt](docs/prompt.md) records the implementation brief. AI assisted the implementation and generated the storefront artwork.

## CSV import

Use **Manage catalog → Import CSV**. The browser sends the file to Catalog, which returns imported and skipped counts and the starting file line, SKU, and reason for each reported issue.

CSV columns must appear in this order.

```csv
name,sku,description,category,price,stock,weight_kg
Ceramic Mug,MUG-001,Stoneware cup,Home,12.50,10,0.285
```

Name, SKU, category, price, stock, and weight are required. Description may be empty. SKUs use ASCII letters, digits, dots, underscores, and hyphens, start with a letter or digit, and are stored in uppercase. The importer accepts UTF-8 and a leading BOM, trims whitespace, and treats `$29.99` as `29.99` and `free` as `0.00`. Whole stock ranges from 0 to 1,000,000 units. Price has up to two decimal places and weight up to three. Price is stored in cents and weight in grams.

The import policy separates file validation from row validation.

- **File validation.** A malformed record, invalid header or encoding, or upload over 1 MiB or 5,000 data records rejects the whole file before any insert. The size and record limits keep parsing and the synchronous import transaction bounded. The Go CSV reader handles quoted commas and multiline fields. [Go CSV reader](https://pkg.go.dev/encoding/csv)
- **Row validation.** Catalog applies the same product validation as manual creation. Valid rows can be imported alongside invalid ones. Invalid field values are reported. Empty physical lines are ignored by the parser. Records whose seven fields are blank count as skipped without an error entry.
- **Duplicate handling.** The first valid row for each normalized SKU is eligible for insertion. An existing active SKU keeps its current values. This insert-only policy lets an operator reimport a corrected file while preserving products and stock that may have changed since the file was prepared.
- **Atomic writes and search updates.** Catalog inserts the accepted products and their search events in one database transaction. A database failure rolls back the accepted batch. The import response confirms the Catalog commit. Storefront results update as Search consumes the events. [Importer and transaction](backend/internal/catalog/import.go)

## Checkout and stock

A cart supports up to 25 distinct products. Each line carries the displayed unit price. Catalog verifies that quote before reserving available stock, so stale quotes and insufficient stock produce a rejection. The USD total is the sum of each unit price times its quantity. An administrator's write uses the version it read. A competing edit or reservation can return `412 Precondition Failed`, which requires loading the current product before retrying.

### Who gets the last unit?

Suppose two buyers each submit a payment choice for a product with one available unit. The first eligible reservation that Catalog commits holds the unit. Catalog locks that product row with `SELECT … FOR UPDATE`, checks price and quantity, and changes available stock from one to zero within the same transaction. A competing reservation processed while the unit is held sees zero and is rejected before payment. Holding an item in the bag or opening the payment page gives neither buyer priority. The ordering that matters is the successful reservation in Catalog. [Inventory reservation](backend/internal/catalog/inventory.go) · [PostgreSQL row locks](https://www.postgresql.org/docs/17/explicit-locking.html#LOCKING-ROWS)

Payment approval settles the reservation. A decline returns the unit to available stock. An order that already received an out-of-stock rejection stays rejected, and a later checkout can reserve the restored unit. The Docker smoke runs two concurrent checkouts against one unit and asserts one paid order, one rejected order, and zero units left when both buyers choose approval. [Settlement](backend/internal/orders/saga.go) · [Concurrent checkout smoke](scripts/smoke.py)

The payment simulator uses a separate page and collects no payment credentials. Order history displays a numeric reference and reads persisted status, including orders awaiting payment. Existing orders gain a reference during migration. Their displayed creation date comes from the migration because the earlier schema stored no timestamp.

## Operations and verification

The [production readiness review](docs/production-readiness.md) records the work needed before public launch.

The stack binds all user traffic to loopback. The gateway accepts local hosts, and API writes with a browser Origin must match that host. Order history is shared within this local demo. Imported names remain data across storage, search and display. The product queries bind values as SQL parameters. React renders text without HTML interpretation. Nginx uses a per-page script policy generated from the Next.js build. A public deployment needs authenticated administration and user-scoped orders, TLS, rotated secrets, database and broker replication, monitoring, backups, and retention for delivery records.

Catalog continues accepting writes during a Search failure. When the broker is down, outbox records accumulate in PostgreSQL. Accepted orders remain pending until the broker and consumers recover. Each consumer commits its inbox record and business change before acknowledging the message. A consumer waits for its database to respond before retrying an interrupted delivery. Repeated failures while the database responds move to a per-service `.dead` queue and can leave an order pending. Search uses the last applied projection while new events are pending.

Inspect pending and dead-letter counts with `docker compose exec broker rabbitmqctl list_queues name messages_ready messages_unacknowledged`. Resolve the underlying error from the service logs before replay. With an AMQP client on the private network, consume the dead-letter message with manual acknowledgement. Republish the original body, content type and message ID to `ecommerce.events` with the original routing key in the first `x-death` entry's `routing-keys` field. Use persistent delivery, mandatory routing and publisher confirms. Acknowledge the dead-letter message after the republish is confirmed and routed. Production recovery after permanent database or broker loss requires coordinated restore and a search projection rebuild from Catalog.

Run the unit checks with Go 1.25 and build the frontend with Node 24.

```sh
go -C backend test -v ./...
go -C backend vet ./...
npm --prefix frontend ci
npm --prefix frontend run build
```

Use the [runtime verification skill](.agents/skills/verify-fieldwork/SKILL.md) for a repeatable Docker check. It creates an isolated project, exercises the real APIs, records the results, and removes the resources it created. Use Python 3 and choose another loopback port with `SHOP_PORT` if needed.

```sh
sh .agents/skills/verify-fieldwork/scripts/verify.sh doctor
sh .agents/skills/verify-fieldwork/scripts/verify.sh
```

The smoke suite checks the four Next.js pages, script policy, exact names from the fixture, product CRUD, persisted pending status, replay, conflicting payment choices, exact totals, and stock compensation. To run it against a separate running test stack, use `python3 scripts/smoke.py --base-url http://127.0.0.1:18081`. Add `--sample` against an empty catalog to include the original fixture.

The [checkout invariants skill](.agents/skills/verify-checkout-invariants/SKILL.md) checks the Orders transition code across duplicate and reordered delivery. Run it after changing checkout states, payment results or compensation commands.

```sh
go -C backend test -count=1 -run '^TestOrderSagaModel$' -v ./internal/orders
```
