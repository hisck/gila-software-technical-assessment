# Checkout and orders

## User path

Add Wireless Mouse and another item in **Storefront**. Open **Bag** and continue to payment. Confirm that a separate browser tab opens. In the market tab, open **Orders** and verify that the new row shows **Pending payment**. Verify the item quantities are available in the catalog while the order is unpaid.

In the simulator tab, approve the payment. Confirm the final amount and **Order confirmed**. Reload the simulator and check that the receipt remains associated with the same order. In **Orders**, verify the numeric reference and final status. The internal UUID stays out of visible status and receipt content.

Create another order, close the simulator, and use the **Orders** screen to reopen it. Decline the simulation and verify that the order is rejected after the product quantities return to their pre-payment values.

## States and checks

- A buyer can resume an unpaid order from history. Stock reservation begins after the buyer submits an outcome on the simulator.
- Equivalent order creation retries resolve to one order. Repeating the same payment choice resolves to the original request. A different choice conflicts.
- A stale quote requires refreshing the storefront and creating a new cart with current prices. Buyers competing for the last unit produce one purchase and one rejection.
- Search for the two supplied fixture entries with SKUs `XS-001` and `SQL-001`. Compare the visible name to the exact CSV and confirm the interface keeps working.
- Use `smoke.log` for API evidence of deep links, script policy, pending history, exact total, compensation and last-unit behavior.
