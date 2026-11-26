# Storefront search

## User path

Open **Storefront** after import. Enter `WIRELESS MOUSE` in **Search products**, select **Electronics**, and submit **Search**. Confirm one Wireless Mouse result at USD 29.99. Clear the term and select **All categories** to verify the paginated list. Enter a unique SKU that matches no product and confirm the empty state.

Change the product in **Manage catalog**, then return to **Storefront** and use **Refresh products** until the card reflects the change. Repeating the same query may use Redis. A product mutation advances the projection revision, which makes the previous cache entry ineligible.

## States and checks

- The fixture proof checks case-insensitive search, category filtering, pagination and exact normalized prices.
- New, updated and deleted products appear after Search applies their events. Give the consumer time to synchronize before comparing results.
- For the HTTP proof, check the normalized fixture and refreshed search lines in `smoke.log`.
