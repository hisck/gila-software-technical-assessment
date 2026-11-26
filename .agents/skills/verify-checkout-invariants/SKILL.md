---
name: verify-checkout-invariants
description: Explore the production Orders saga state space and report a counterexample when checkout, payment or compensation transitions violate an invariant.
---

# Verify checkout invariants

Run this check after changes to checkout status, inventory replies, payment replies or outbound compensation commands. Use Go 1.25 from the repository root.

```sh
go -C backend test -count=1 -run '^TestOrderSagaModel$' -v ./internal/orders
```

The command must run `TestOrderSagaModel`. The current model reaches 7 states and checks 35 deliveries across 5 inbound topics. A changed count requires explaining the intended new path. A failed check prints the shortest known event trace to the violated invariant.

## Model and scope

The model checks a new order in `awaiting_payment` and an order in `pending_inventory` after the [payment handler](../../../backend/internal/orders/payment.go) has accepted the outcome. The order [creation handler](../../../backend/internal/orders/orders.go) establishes the unpaid state. At each state it applies every inbound topic to the same `orderTransition` function used by [saga.go](../../../backend/internal/orders/saga.go). The model records whether Orders has requested payment and stock release. Its state bound is six statuses times two binary history variables. Revisiting a state with the same history adds no cases, so this covers arbitrary-length sequences of the five topics, including duplicate and reordered deliveries.

| Promise | Checked behavior |
| --- | --- |
| Unpaid orders ignore broker replies | No topic changes `awaiting_payment` or emits a command. |
| A payment request follows a reservation | The transition to `pending_payment` emits one payment command after the inventory reservation reply. |
| Stock commit follows approval | An approved, requested payment moves an order to `paid` and emits the commit command. |
| Decline waits for compensation | A decline emits one release command. Orders becomes `rejected` after the release confirmation. |
| Terminal outcomes are stable | Later replies neither change the status nor emit another command. |
| The expected paths exist | Paid, inventory rejection and compensated decline are reachable. |

This checks the selector and command classes. The Docker smoke checks the HTTP transition into `pending_inventory`, concurrent outcomes and durable history. It assumes a known order and trusted producers. The Docker smoke covers actual SQL transactions, order snapshots, outbox delivery, payment persistence and stock changes. The model omits Redis, SQL isolation, broker delivery, network failures and cross-order stock contention.

## Counterexample and maintenance

During authoring on Go 1.25.4, an isolated copy changed the decline transition to return terminal `rejected` immediately. The test failed with `a declined payment became final before stock release was confirmed`. The trace was `pending_inventory → inventory.reserved.v1 → pending_payment → payment.declined.v1 → rejected`. Restoring the selector passed the original check. During the payment split, a scratch mutation accepted `inventory.reserved.v1` in `awaiting_payment`. The model reported an unauthorized transition to `pending_payment` and a payment command. Restoring the guard passed with seven states and 35 deliveries.

For future changes, update the expected promises from the requirements and verify the real selector. Run a deliberate failing variant in an isolated copy, then rerun the baseline after restoring it.

Source synchronization is recorded by SHA-256 for this initial working tree. Synchronize the references with the next commit that changes these files.

- `saga.go` · `c4621ba5218218d103fefa85252975ff706f89aaece2129dc795ff11252e29ad`
- `saga_model_test.go` · `2ff0b20bca7ef9a6c1ec5ded6c930d6e43d0a43184924c08462203ef95e8ca04`
