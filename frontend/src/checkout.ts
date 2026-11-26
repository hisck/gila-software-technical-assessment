import { useCallback, useEffect, useRef, useState } from "react"
import { api, APIError } from "./api"
import type { CheckoutItem, CheckoutRequest, Order, Product } from "./api"

export type CartLine = { product: Product; quantity: number }
type Attempt = { key: string; request: CheckoutRequest; id?: string }
type Cart = { lines: CartLine[]; attempt: Attempt | null; reviewPriorOrder?: boolean }
const emptyCart: Cart = { lines: [], attempt: null }
const storageKey = "fieldwork.cart.v2"
const maxCartLines = 25
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const pricePattern = /^[0-9]{1,9}(\.[0-9]{1,2})?$/

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isUUID(value: unknown): value is string {
  return typeof value === "string" && uuidPattern.test(value) && value !== "00000000-0000-0000-0000-000000000000"
}

function isPrice(value: unknown): value is string {
  return typeof value === "string" && pricePattern.test(value)
}

function isQuantity(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= 1_000_000
}

function isCartLine(value: unknown): value is CartLine {
  if (!isRecord(value) || !isQuantity(value.quantity) || !isRecord(value.product)) return false
  const { id, name, sku, description, category, price, stock, weight_kg, version } = value.product
  return isUUID(id) && typeof name === "string" && typeof sku === "string" &&
    typeof description === "string" && typeof category === "string" &&
    isPrice(price) && (stock === 0 || isQuantity(stock)) &&
    typeof weight_kg === "string" && typeof version === "number" && Number.isSafeInteger(version)
}

function isCheckoutItem(value: unknown): value is CheckoutItem {
  return isRecord(value) && isUUID(value.product_id) && isQuantity(value.quantity) && isPrice(value.unit_price)
}

function readAttempt(value: unknown): Attempt | null {
  if (!isRecord(value) || !isUUID(value.key) || !isRecord(value.request)) return null
  const { id } = value
  const { items } = value.request
  if (id !== undefined && !isUUID(id)) return null
  if (!Array.isArray(items) || !items.length || items.length > maxCartLines || !items.every(isCheckoutItem)) return null
  return {
    key: value.key,
    id,
    request: { items: items.map(({ product_id, quantity, unit_price }) => ({ product_id, quantity, unit_price })) },
  }
}

function loadCart(): Cart {
  try {
    const saved = sessionStorage.getItem(storageKey)
    const raw = saved ?? sessionStorage.getItem("fieldwork.cart.v1")
    if (raw === null) return emptyCart
    const stored: unknown = JSON.parse(raw)
    if (!isRecord(stored)) return emptyCart

    const lines = Array.isArray(stored.lines) && stored.lines.length <= maxCartLines && stored.lines.every(isCartLine)
      ? stored.lines
      : []
    const attempt = readAttempt(stored.attempt)
    if (attempt?.id) return { lines: [], attempt }
    if (saved !== null && attempt) {
      const { items } = attempt.request
      const matches = lines.length === items.length && lines.every(({ product, quantity }, index) => {
        const item = items[index]
        return item.product_id === product.id && item.quantity === quantity && item.unit_price === product.price
      })
      return { lines: matches ? lines : [], attempt }
    }
    return { lines, attempt: null, reviewPriorOrder: stored.reviewPriorOrder === true || stored.attempt != null }
  } catch {
    return emptyCart
  }
}

export const statusLabels: Record<Order["status"], string> = {
  awaiting_payment: "Pending payment",
  pending_inventory: "Confirming stock",
  pending_payment: "Processing simulated payment",
  releasing_inventory: "Restoring stock",
  paid: "Order confirmed",
  rejected: "Order declined",
}

export function orderFinished(order?: Order) {
  return order?.status === "paid" || order?.status === "rejected"
}

export function cents(price: string): bigint {
  const [whole, fraction = ""] = price.split(".")
  return BigInt(whole) * 100n + BigInt(fraction.padEnd(2, "0"))
}

const integer = new Intl.NumberFormat("en-US")

export function usd(value: string | bigint) {
  const amount = typeof value === "string" ? cents(value) : value
  return `$${integer.format(amount / 100n)}.${String(amount % 100n).padStart(2, "0")}`
}

export function orderTotal(order: Order) {
  return usd(order.total ?? order.items.reduce((sum, item) => sum + cents(item.unit_price) * BigInt(item.quantity), 0n))
}

export function paymentHref(id: string) {
  return `/payment/#${encodeURIComponent(id)}`
}

export function useOrderStatus(id?: string) {
  const [result, setResult] = useState<{ id: string; order?: Order; error?: string }>()
  const [revision, setRevision] = useState(0)

  useEffect(() => {
    if (!id) return
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout> | undefined

    async function check(orderId: string) {
      try {
        const order = await api.order(orderId, controller.signal)
        if (controller.signal.aborted) return
        setResult({ id: orderId, order })
        if (!orderFinished(order)) timer = setTimeout(() => void check(orderId), 950)
      } catch (failure) {
        if (controller.signal.aborted) return
        const missing = failure instanceof APIError && failure.status === 404
        const error = missing ? "This order could not be found." : "Order status is temporarily unavailable. Checking again automatically."
        setResult((previous) => ({ id: orderId, order: previous?.id === orderId ? previous.order : undefined, error }))
        if (!missing) timer = setTimeout(() => void check(orderId), 3000)
      }
    }

    void check(id)
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [id, revision])

  return {
    order: result?.id === id ? result?.order : undefined,
    error: result?.id === id ? result?.error : undefined,
    refresh: () => setRevision((value) => value + 1),
  }
}

export function useCart() {
  const [state, setState] = useState(emptyCart)
  const current = useRef(state)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const sending = useRef(false)
  const { order, error: statusError } = useOrderStatus(state.attempt?.id)

  useEffect(() => {
    const stored = loadCart()
    current.current = stored
    setState(stored)
  }, [])

  const save = useCallback((next: Cart, requirePersistence = false) => {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(next))
    } catch {
      if (requirePersistence) throw new Error("Enable session storage in this browser before checking out.")
    }
    current.current = next
    setState(next)
  }, [])

  function add(product: Product) {
    const latest = current.current
    if (latest.attempt && !latest.attempt.id) return "Open your bag to retry the pending checkout."
    const existing = latest.lines.find((line) => line.product.id === product.id)
    if (product.stock <= (existing?.quantity ?? 0)) return "You already have the available quantity in your bag."
    if (!existing && latest.lines.length >= maxCartLines) return `Your bag can hold up to ${maxCartLines} different products.`

    save({
      ...latest,
      lines: existing
        ? latest.lines.map((line) => line === existing ? { product, quantity: line.quantity + 1 } : line)
        : [...latest.lines, { product, quantity: 1 }],
      attempt: null,
    })
    setError("")
  }

  function changeQuantity(id: string, quantity: number) {
    const latest = current.current
    if (latest.attempt) return
    save({
      ...latest,
      lines: latest.lines.flatMap((line) => {
        if (line.product.id !== id) return [line]
        return quantity <= 0 ? [] : [{ ...line, quantity: Math.min(quantity, line.product.stock) }]
      }),
    })
  }

  function finishAttempt() {
    setError("")
    save({ ...current.current, attempt: null })
  }

  function acknowledgePriorOrder() {
    save({ ...current.current, reviewPriorOrder: false })
  }

  async function submit() {
    const latest = current.current
    if (sending.current || latest.attempt?.id || latest.reviewPriorOrder || (!latest.attempt && !latest.lines.length)) return
    sending.current = true
    setSubmitting(true)
    setError("")
    try {
      const attempt = latest.attempt ?? {
        key: crypto.randomUUID(),
        request: {
          items: latest.lines.map(({ product, quantity }) => ({ product_id: product.id, quantity, unit_price: product.price })),
        },
      }
      save({ ...latest, attempt }, true)
      const { id } = await api.checkout(attempt.request, attempt.key)
      save({ lines: [], attempt: { ...attempt, id } })
      return id
    } catch (failure) {
      if (failure instanceof APIError && [400, 409, 422].includes(failure.status)) {
        save({ ...latest, attempt: null })
      }
      setError(failure instanceof Error ? failure.message : "Checkout is temporarily unavailable.")
    } finally {
      sending.current = false
      setSubmitting(false)
    }
  }

  const total = state.lines.reduce((value, line) => value + cents(line.product.price) * BigInt(line.quantity), 0n)
  return { ...state, order, total, submitting, error: error || statusError, add, changeQuantity, finishAttempt, acknowledgePriorOrder, submit }
}
