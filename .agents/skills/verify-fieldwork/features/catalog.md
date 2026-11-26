# Catalog and CSV import

## User path

Open **Manage catalog**. Use **Add product** and enter a unique SKU with price, stock and weight. Find the row by name and record the product ID from the create response. Compare the SKU and stock with the Catalog API result. Open **Edit** and change the price. Use **Delete** and confirm **Delete product**. Verify the row disappears and `GET /api/catalog/products/{id}` returns 404.

Open **Import CSV**, select **Download sample CSV**, choose the downloaded file in **Product CSV**, and submit **Import CSV**. Capture the report with 87 imported rows, 10 skipped rows and 8 issues. Reimport the same file and confirm zero new products with unchanged values.

## States and checks

- An existing SKU produces a conflict. A concurrent stock change makes an old edit fail and asks for the current product.
- A malformed CSV rejects the request before an insert. Invalid values in a well-formed row appear as line issues.
- For the HTTP proof, read the fixture, CRUD, and deletion lines in `smoke.log`. Catalog is authoritative while Storefront updates arrive through the broker.
