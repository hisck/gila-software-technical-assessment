"use client"

import { ClipboardList, ExternalLink, RefreshCw } from "lucide-react"
import Link from "next/link"
import { useEffect, useState } from "react"
import { api, type OrderPage } from "../api"
import { orderTotal, paymentHref, statusLabels } from "../checkout"
import { PageControls } from "./PageControls"
import { Badge } from "./ui/badge"
import { Button } from "./ui"

const orderDate = new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short" })

export function OrderHistory() {
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [result, setResult] = useState<OrderPage>()
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout> | undefined
    setLoading(true)

    async function load() {
      try {
        const data = await api.orders(page, controller.signal)
        if (controller.signal.aborted) return
        const lastPage = Math.max(1, Math.ceil(data.total / data.limit))
        if (page > lastPage) {
          setPage(lastPage)
          return
        }
        setResult(data)
        setError("")
      } catch (failure) {
        if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : "Could not load orders.")
      }
      if (controller.signal.aborted) return
      setLoading(false)
      timer = setTimeout(() => void load(), 2500)
    }

    void load()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [page, revision])

  return (
    <main id="main-content" className="page-shell admin-page orders-page" tabIndex={-1}>
      <header className="admin-heading">
        <div>
          <h1>Orders</h1>
          <p>Review purchases and resume pending payments.</p>
        </div>
        <Button variant="outline" disabled={loading} onClick={() => setRevision((value) => value + 1)}>
          <RefreshCw /> Refresh
        </Button>
      </header>
      {error && <p role="alert" className="feedback feedback-error">{error}</p>}
      {loading && !result && <p role="status" className="results-summary">Loading orders…</p>}
      {result?.items.length ? (
        <>
          <ol className="orders-list" aria-label="Order history" aria-busy={loading}>
            {result.items.map((order) => {
              const quantity = order.items.reduce((sum, item) => sum + item.quantity, 0)
              const containsNames = order.items.some((item) => item.name || item.sku)
              return (
                <li key={order.id} className="order-entry">
                  <div>
                    <h2>Order #{order.number}</h2>
                    <time dateTime={order.created_at}>{orderDate.format(new Date(order.created_at))}</time>
                  </div>
                  <div className="order-entry-items">
                    <p>{quantity.toLocaleString("en-US")} {quantity === 1 ? "item" : "items"}</p>
                    {containsNames && (
                      <ul aria-label="Products">
                        {order.items.map((item) => (
                          <li key={item.product_id}>{item.quantity} × {item.name ?? item.sku ?? "Product"}</li>
                        ))}
                      </ul>
                    )}
                    {order.reason && <p>{order.reason}</p>}
                  </div>
                  <div className="order-entry-total">
                    <strong>{orderTotal(order)}</strong>
                    {order.total === undefined && <small>Estimated total</small>}
                  </div>
                  <div className="order-entry-status">
                    <Badge className={`status-badge ${order.status}`}>{statusLabels[order.status]}</Badge>
                    {order.status === "awaiting_payment" && (
                      <Button variant="outline" size="small" asChild>
                        <a href={paymentHref(order.id)} target="_blank" rel="noopener noreferrer">Open payment <ExternalLink /></a>
                      </Button>
                    )}
                  </div>
                </li>
              )
            })}
          </ol>
          <PageControls page={result.page} limit={result.limit} total={result.total} onChange={setPage} label="Order pages" />
        </>
      ) : result && (
        <div className="empty-state">
          <ClipboardList />
          <h2>No orders yet</h2>
          <p>Your orders will appear here once you start checkout.</p>
          <Button asChild><Link href="/">Browse products</Link></Button>
        </div>
      )}
    </main>
  )
}
