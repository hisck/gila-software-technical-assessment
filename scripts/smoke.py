import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
import csv
from datetime import datetime
from hashlib import sha256
import json
from pathlib import Path
import re
import time
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import Request, urlopen
from uuid import uuid4


CATALOG = "/api/catalog/products"
SEARCH = "/api/search/products"
ORDERS = "/api/orders"
FIXTURE = Path(__file__).resolve().parents[1] / "Code Challenge E-Commerce.csv"


class Shop:
    def __init__(self, base_url):
        self.base_url = base_url.rstrip("/")

    def call(self, path, status=200, *, method="GET", body=None, headers=None):
        headers = dict(headers or {})
        if isinstance(body, dict):
            body = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        request = Request(self.base_url + path, body, headers, method=method)
        try:
            response = urlopen(request, timeout=15)
        except HTTPError as failure:
            response = failure
        with response:
            raw = response.read()
            assert response.status == status, (method, path, status, response.status, raw[:500])
            return json.loads(raw) if raw else None

    def search(self, **filters):
        return self.call(SEARCH + "?" + urlencode(filters))

    def product(self, product):
        return self.call(f"{CATALOG}/{product['id']}")

    def import_csv(self, content, status=200):
        return self.call("/api/catalog/import", status, method="POST", body=content,
                         headers={"Content-Type": "text/csv"})

    def submit(self, body, key=None, status=202):
        return self.call(ORDERS, status, method="POST", body=body,
                         headers={"Idempotency-Key": str(key or uuid4())})

    def order(self, order):
        return self.call(f"{ORDERS}/{order['id']}")

    def pay(self, order, outcome, status=202):
        return self.call(f"{ORDERS}/{order['id']}/payment", status, method="POST",
                         body={"outcome": outcome})

    def finished(self, order):
        return eventually(lambda: self.order(order), lambda data: data["status"] in {"paid", "rejected"})


def eventually(fetch, condition):
    deadline = time.monotonic() + 35
    while time.monotonic() < deadline:
        data = fetch()
        if condition(data):
            return data
        time.sleep(0.25)
    raise AssertionError(f"Timed out waiting for state. Last response: {data}")


def line(product, quantity=1):
    return {"product_id": product["id"], "quantity": quantity, "unit_price": product["price"]}


def check_pages(shop):
    for route in ("/", "/catalog/", "/orders/", "/payment/"):
        with urlopen(shop.base_url + (route.rstrip("/") or "/"), timeout=15) as response:
            assert response.geturl() == shop.base_url + route, response.geturl()
            html = response.read().decode()
            policy = response.headers["Content-Security-Policy"]
            assert response.headers["X-Content-Type-Options"] == "nosniff", route
        script_policy = next(directive.split() for directive in policy.split(";")
                             if directive.strip().startswith("script-src "))
        assert "'self'" in script_policy
        assert "'unsafe-inline'" not in script_policy and "'unsafe-eval'" not in script_policy
        scripts = re.findall(r"<script\b([^>]*)>([\s\S]*?)</script\s*>", html, re.I)
        inline = [body for attributes, body in scripts if body and not re.search(r"\ssrc\s*=", attributes, re.I)]
        for body in inline:
            digest = base64.b64encode(sha256(body.encode()).digest()).decode()
            assert f"'sha256-{digest}'" in script_policy, (route, digest)
        assert "/_next/static/" in html, route
    print("PASS four exported Next.js routes and build-specific script policy")


def check_fixture(shop):
    report = shop.import_csv(FIXTURE.read_bytes())
    assert (report["imported"], report["skipped"]) == (87, 10), report
    assert [issue["line"] for issue in report["issues"]] == [16, 25, 36, 41, 50, 52, 56, 89], report
    projected = eventually(lambda: shop.search(), lambda page: page["total"] == 87)
    assert len(projected["items"]) == 12 and projected["limit"] == 12
    filters = shop.search(q="WIRELESS MOUSE", category="Electronics")
    assert filters["total"] == 1 and filters["items"][0]["price"] == "29.99", filters
    free = shop.search(q="YM-015")
    assert free["items"][0]["price"] == "0.00", free
    duplicate = shop.search(q="RS-001")
    assert duplicate["total"] == 1 and duplicate["items"][0]["price"] == "89.99", duplicate
    with FIXTURE.open(encoding="utf-8", newline="") as file:
        names = {row["sku"]: row["name"] for row in csv.DictReader(file)}
    for sku in ("XS-001", "SQL-001"):
        literal = names[sku]
        result = shop.search(q=literal)
        assert result["total"] == 1 and result["items"][0]["sku"] == sku, result
        assert result["items"][0]["name"] == literal
        assert shop.product(result["items"][0])["name"] == literal
    retry = shop.import_csv(FIXTURE.read_bytes())
    assert retry["imported"] == 0 and retry["skipped"] == 97, retry
    print("PASS original CSV, literal fixture names, filtering and non-destructive reimport")


def check_shop(shop):
    token = uuid4().hex[:10]
    fields = {"name": f"Market smoke {token}", "sku": f"SMK-{token}",
              "category": "Test", "description": "Smoke test item",
              "price": "12.30", "stock": 4, "weight_kg": "0.125"}
    shop.call(CATALOG, 400, method="POST", body={**fields, "stock": -1})
    shop.call(CATALOG, 403, method="POST", body=fields, headers={"Origin": "https://example.com"})
    item = shop.call(CATALOG, 201, method="POST", body=fields)
    assert item["sku"] == fields["sku"].upper()
    shop.call(CATALOG, 409, method="POST", body=fields)
    item_path = f"{CATALOG}/{item['id']}"
    assert shop.product(item) == item
    visible = eventually(lambda: shop.search(q=token), lambda page: page["total"] == 1)
    assert visible["items"][0] == item

    new_fields = {**fields, "price": "12.35", "name": fields["name"] + " updated"}
    version = {"If-Match": f"\"{item['version']}\""}
    shop.call(item_path, 428, method="PUT", body=new_fields)
    updated = shop.call(item_path, method="PUT", body=new_fields, headers=version)
    assert updated["version"] == item["version"] + 1 and updated["price"] == "12.35"
    shop.call(item_path, 412, method="PUT", body=fields, headers=version)
    projection = eventually(lambda: shop.search(q=token),
                            lambda page: page["items"][0]["version"] == updated["version"])
    assert projection["items"][0]["price"] == "12.35"

    csv = ("name,sku,description,category,price,stock,weight_kg\n"
           f"Atomic candidate,AT-{token},Candidate,Test,1.00,1,0.0\n"
           "\"unfinished").encode()
    shop.import_csv(csv, status=400)
    assert shop.call(CATALOG + "?" + urlencode({"q": f"AT-{token}"}))["total"] == 0
    print("PASS product create/read/update, duplicate SKU, stale edit, CSV atomic syntax and refreshed search")

    drafts = [shop.submit({"items": [line(updated)]}) for _ in range(10)]
    history = shop.call(ORDERS + "?page=1&limit=10")
    assert [item["id"] for item in history["items"]] == [item["id"] for item in reversed(drafts)], history
    assert all(item["status"] == "awaiting_payment" for item in history["items"])
    assert shop.product(item)["stock"] == 4

    stale = shop.submit({"items": [line(item)]})
    shop.pay(stale, "approve")
    stale_order = shop.finished(stale)
    assert stale_order["status"] == "rejected" and "price" in stale_order["reason"].lower()
    assert shop.product(item)["stock"] == 4

    key = uuid4()
    request = {"items": [line(updated, 2)]}
    with ThreadPoolExecutor(max_workers=8) as pool:
        attempts = list(pool.map(lambda _: shop.submit(request, key), range(8)))
    assert len({attempt["id"] for attempt in attempts}) == 1, attempts
    shop.submit({"items": [line(updated)]}, key, status=409)
    waiting = shop.order(attempts[0])
    assert waiting["status"] == "awaiting_payment", waiting
    assert waiting["number"].isdigit() and datetime.fromisoformat(waiting["created_at"].replace("Z", "+00:00")).tzinfo
    assert shop.product(item)["stock"] == 4
    history = shop.call(ORDERS + "?page=1&limit=1")
    assert history["items"] == [waiting] and history["total"] >= 2 and history["limit"] == 1, history
    shop.pay(waiting, "invalid", status=400)
    assert shop.order(waiting)["status"] == "awaiting_payment"
    with ThreadPoolExecutor(max_workers=8) as pool:
        payments = list(pool.map(lambda _: shop.pay(waiting, "approve"), range(8)))
    assert all(payment == attempts[0] for payment in payments), payments
    shop.pay(waiting, "decline", status=409)
    paid = shop.finished(attempts[0])
    assert paid["status"] == "paid" and paid["total"] == "24.70", paid
    assert paid["number"] == waiting["number"]
    assert paid["items"][0]["name"] == updated["name"]
    assert shop.product(item)["stock"] == 2
    print("PASS numeric pending order history, concurrent retries, conflicting payment and exact total")

    second = shop.call(CATALOG, 201, method="POST",
                       body={**fields, "sku": f"SMK-SECOND-{token}", "stock": 1})
    missing = shop.submit({"items": [line(updated), line(second, 2)]})
    shop.pay(missing, "approve")
    assert shop.finished(missing)["status"] == "rejected"
    assert shop.product(item)["stock"] == 2 and shop.product(second)["stock"] == 1

    declined = shop.submit({"items": [line(updated), line(second)]})
    shop.pay(declined, "decline")
    rejection = shop.finished(declined)
    assert rejection["status"] == "rejected" and "declined" in rejection["reason"]
    assert shop.product(item)["stock"] == 2 and shop.product(second)["stock"] == 1

    scarce = {"items": [line(second)]}
    with ThreadPoolExecutor(max_workers=2) as pool:
        orders = list(pool.map(lambda _: shop.pay(shop.submit(scarce), "approve"), range(2)))
    results = [shop.finished(order) for order in orders]
    assert sorted(result["status"] for result in results) == ["paid", "rejected"], results
    assert shop.product(second)["stock"] == 0
    print("PASS atomic multi-product reservation, completed compensation and no overselling")

    old = shop.product(item)
    shop.call(item_path, 204, method="DELETE", headers={"If-Match": f"\"{old['version']}\""})
    shop.call(item_path, 404)
    replacement = shop.call(CATALOG, 201, method="POST", body=fields)
    assert replacement["id"] != item["id"]
    matches = eventually(lambda: shop.search(q=fields["sku"]), lambda page:
                         len(page["items"]) == 1 and page["items"][0]["id"] == replacement["id"])
    assert matches["items"][0]["stock"] == fields["stock"]

    print("PASS delete, SKU reuse and search tombstone")


if __name__ == "__main__":
    if not __debug__:
        raise RuntimeError("Smoke checks require enabled assertions")
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--sample", action="store_true", help="Import the original fixture into an empty project")
    options = parser.parse_args()
    shop = Shop(options.base_url)
    check_pages(shop)
    if options.sample:
        check_fixture(shop)
    check_shop(shop)
