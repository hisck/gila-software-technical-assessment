export type ProductFields = {
  name: string
  sku: string
  description: string
  category: string
  price: string
  stock: number
  weight_kg: string
}

export type Product = ProductFields & { id: string; version: number }

export type ProductPage = {
  items: Product[]
  total: number
  page: number
  limit: number
}

export type SearchPage = ProductPage & { categories: string[] }

export type ImportReport = {
  imported: number
  skipped: number
  issues: { line: number; sku: string; message: string }[]
}

export type PaymentOutcome = "approve" | "decline"

export type CheckoutItem = {
  product_id: string
  quantity: number
  unit_price: string
}

export type CheckoutRequest = {
  items: CheckoutItem[]
}

export type Order = {
  id: string
  number: string
  created_at: string
  status: "awaiting_payment" | "pending_inventory" | "pending_payment" | "releasing_inventory" | "paid" | "rejected"
  reason?: string
  total?: string
  items: (CheckoutItem & { sku?: string; name?: string })[]
}

export type OrderPage = Omit<ProductPage, "items"> & { items: Order[] }

export class APIError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const timeout = AbortSignal.timeout(15_000)
  const signal = init.signal ? AbortSignal.any([init.signal, timeout]) : timeout
  let response: Response
  try {
    response = await fetch(path, { ...init, signal })
  } catch (error) {
    if (init.signal?.aborted) throw error
    throw new Error("The service could not be reached. Please try again.")
  }

  if (response.status === 204) return undefined as T
  const result: unknown = await response.json().catch(() => null)
  if (!response.ok) {
    const message = result && typeof result === "object" && "error" in result && typeof result.error === "string"
      ? result.error
      : "The service could not complete this request. Please try again."
    throw new APIError(message, response.status)
  }
  if (result === null) throw new Error("The service returned an unreadable response. Please try again.")
  return result as T
}

const productPath = "/api/catalog/products"
const orderPath = "/api/orders"

function versionHeader(version: number) {
  return { "If-Match": `"${version}"` }
}

export const api = {
  search(query: string, category: string, page: number, signal: AbortSignal) {
    const params = new URLSearchParams({ q: query, category, page: String(page), limit: "12" })
    return request<SearchPage>(`/api/search/products?${params}`, { signal })
  },
  products(page: number, signal: AbortSignal) {
    return request<ProductPage>(`${productPath}?page=${page}&limit=12`, { signal })
  },
  product(id: string, signal: AbortSignal) {
    return request<Product>(`${productPath}/${id}`, { signal })
  },
  saveProduct(fields: ProductFields, existing?: Product) {
    return request<Product>(existing ? `${productPath}/${existing.id}` : productPath, {
      method: existing ? "PUT" : "POST",
      headers: { "Content-Type": "application/json", ...(existing ? versionHeader(existing.version) : {}) },
      body: JSON.stringify(fields),
    })
  },
  deleteProduct(product: Product) {
    return request<void>(`${productPath}/${product.id}`, { method: "DELETE", headers: versionHeader(product.version) })
  },
  importProducts(file: File) {
    return request<ImportReport>("/api/catalog/import", {
      method: "POST",
      headers: { "Content-Type": "text/csv" },
      body: file,
    })
  },
  checkout(body: CheckoutRequest, key: string) {
    return request<{ id: string }>(orderPath, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": key },
      body: JSON.stringify(body),
    })
  },
  order(id: string, signal: AbortSignal) {
    return request<Order>(`${orderPath}/${id}`, { signal })
  },
  orders(page: number, signal: AbortSignal) {
    return request<OrderPage>(`${orderPath}?page=${page}&limit=12`, { signal })
  },
  pay(id: string, outcome: PaymentOutcome) {
    return request<{ id: string }>(`${orderPath}/${id}/payment`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ outcome }),
    })
  },
}
