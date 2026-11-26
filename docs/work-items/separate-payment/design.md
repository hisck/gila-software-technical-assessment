# Design

Use React and locally owned shadcn/ui components in Next.js App Router. Export the storefront, catalog, orders and payment pages to static HTML. Nginx serves deep links and the existing Go APIs. An alternative Next.js server mode would require a Node runtime and offer per-request rendering. This application uses interactive client-side API requests with a shared market layout. The payment route uses a separate provider layout and no payment credentials.

| HTTP | Contract |
| --- | --- |
| `POST /api/orders` | Body `{items:[{product_id,quantity,unit_price}]}` and UUID `Idempotency-Key`. Returns 202 with the internal `id`. Stores a durable `awaiting_payment` order and no stock reservation. |
| `POST /api/orders/{id}/payment` | Body `{outcome:"approve"|"decline"}`. Returns 202 and `id`. Atomically records the choice, moves to `pending_inventory` and schedules stock reservation. An identical retry returns the same result. A conflicting choice is 409. |
| `GET /api/orders/{id}` | Returns `id`, decimal-string `number`, ISO `created_at`, `status`, `items`, optional `total` and `reason`. |
| `GET /api/orders?page=&limit=` | Returns `{items,total,page,limit}` in descending order number. The local demo shows its shared history. |

Status labels distinguish pending payment, stock confirmation, payment processing, stock restoration and terminal states. Once chosen, the existing RabbitMQ saga validates current price and stock, processes the simulator decision, and releases on decline. Holding stock while the buyer has the provider tab open would require expiration and abandoned-order compensation. Starting the stock reservation on provider submission avoids that hold. An unpaid abandoned order remains visible and resumable. Database migration keeps prior orders and pending saga messages valid.

A new browser tab opens from the checkout click. The local payment page uses the internal order ID to load persisted state and displays a numeric order reference where useful. Browser URLs and visible status copy use a clean provider page after capturing the handoff. Existing orders and new orders use the same payment microservice through RabbitMQ.

Product names remain unchanged as data. React renders them in text nodes. PostgreSQL queries bind user-controlled values as parameters. The static export hashes its own inline bootstrapping scripts for CSP and continues to reject unapproved scripts. Tests use the supplied CSV fixture and verify the literal UI, catalog schema and subsequent writes.
