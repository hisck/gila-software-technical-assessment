"use client"

import Link from "next/link"
import { ChevronRight, Package, Pencil, Plus, RefreshCw, Trash2, Upload } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import type { MouseEvent } from "react"
import { api, type Product, type ProductPage } from "../api"
import { usd } from "../checkout"
import { DeleteProduct, ImportProducts, ProductEditor } from "./CatalogDialogs"
import { PageControls } from "./PageControls"
import { Button, DialogRoot } from "./ui"

type Selection = { kind: "create" | "import" } | { kind: "edit" | "delete"; product: Product }

export function CatalogAdmin() {
  const [page, setPage] = useState(1)
  const [result, setResult] = useState<ProductPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [revision, setRevision] = useState(0)
  const [selection, setSelection] = useState<Selection | null>(null)
  const [busy, setBusy] = useState(false)
  const [notification, setNotification] = useState("")
  const previousFocus = useRef<HTMLButtonElement | null>(null)
  const addProductButton = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError("")

    void api.products(page, controller.signal)
      .then((data) => {
        if (controller.signal.aborted) return
        const lastPage = Math.max(1, Math.ceil(data.total / data.limit))
        if (page > lastPage) {
          setPage(lastPage)
          return
        }
        setResult(data)
        setLoading(false)
      })
      .catch((failure: Error) => {
        if (controller.signal.aborted) return
        setError(failure.message)
        setLoading(false)
      })

    return () => controller.abort()
  }, [page, revision])

  function refresh() {
    setRevision((value) => value + 1)
  }

  function open(event: MouseEvent<HTMLButtonElement>, next: Selection) {
    previousFocus.current = event.currentTarget
    setNotification("")
    setSelection(next)
  }

  function restoreFocus(event: Event) {
    event.preventDefault()
    const trigger = previousFocus.current
    if (trigger?.isConnected) trigger.focus()
    else addProductButton.current?.focus()
  }

  const dialogContext = { busy, setBusy, restoreFocus }

  return (
    <main id="main-content" className="page-shell admin-page" tabIndex={-1}>
      <nav className="breadcrumb" aria-label="Breadcrumb">
        <Link href="/">Storefront</Link><ChevronRight /><span aria-current="page">Manage catalog</span>
      </nav>
      <header className="admin-heading">
        <div>
          <h1>Product catalog</h1>
          <p>Manage product details, inventory, and CSV imports.</p>
        </div>
        <div className="admin-actions">
          <Button variant="outline" onClick={(event) => open(event, { kind: "import" })}><Upload /> Import CSV</Button>
          <Button ref={addProductButton} onClick={(event) => open(event, { kind: "create" })}><Plus /> Add product</Button>
        </div>
      </header>

      {notification && <p className="feedback feedback-success" role="status">{notification}</p>}
      {error && <p className="feedback feedback-error" role="alert">{error}</p>}

      <section className="admin-list" aria-labelledby="catalog-list-title">
        <header className="admin-list-header">
          <div>
            <h2 id="catalog-list-title">Products</h2>
            <p className="muted" role="status">{loading ? "Loading products…" : result ? `${result.total.toLocaleString("en-US")} ${result.total === 1 ? "product" : "products"}` : ""}</p>
          </div>
          <Button variant="outline" size="icon" aria-label="Refresh catalog" title="Refresh catalog" disabled={loading} onClick={refresh}><RefreshCw /></Button>
        </header>

        {result?.items.length ? (
          <div className="table-frame" aria-busy={loading}>
            <table className="product-table" role="table">
              <caption className="sr-only">Products in the catalog</caption>
              <thead><tr><th scope="col">Product</th><th scope="col">Category</th><th scope="col">Price</th><th scope="col">Stock</th><th scope="col">Actions</th></tr></thead>
              <tbody>
                {result.items.map((product) => (
                  <tr key={product.id}>
                    <th scope="row"><strong>{product.name}</strong><span className="sku">{product.sku}</span></th>
                    <td>{product.category}</td>
                    <td className="price-cell">{usd(product.price)}</td>
                    <td className={product.stock ? "stock-cell in-stock" : "stock-cell out-stock"}>{product.stock.toLocaleString("en-US")}</td>
                    <td className="row-actions">
                      <Button size="small" variant="outline" aria-label={`Edit ${product.name}`} onClick={(event) => open(event, { kind: "edit", product })}><Pencil /> Edit</Button>
                      <Button size="icon" variant="ghost" aria-label={`Delete ${product.name}`} title={`Delete ${product.name}`} onClick={(event) => open(event, { kind: "delete", product })}><Trash2 /></Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : result && !loading ? (
          <div className="empty-state admin-empty">
            <Package />
            <h3>No products yet</h3>
            <p>Add a product or import a CSV to populate your catalog.</p>
          </div>
        ) : null}

        {result && <PageControls page={result.page} total={result.total} limit={result.limit} onChange={setPage} />}
      </section>

      <DialogRoot open={!!selection} onOpenChange={(open) => { if (!open && !busy) setSelection(null) }}>
        {selection?.kind === "create" || selection?.kind === "edit" ? (
          <ProductEditor {...dialogContext} id={selection.kind === "edit" ? selection.product.id : undefined} onSaved={(product) => {
            setSelection(null)
            setNotification(`${product.name} was ${selection.kind === "create" ? "added" : "updated"}.`)
            refresh()
          }} />
        ) : selection?.kind === "delete" ? (
          <DeleteProduct {...dialogContext} product={selection.product} onDeleted={() => {
            setSelection(null)
            setNotification(`${selection.product.name} was deleted.`)
            refresh()
          }} />
        ) : selection?.kind === "import" ? (
          <ImportProducts {...dialogContext} onImported={() => { setPage(1); refresh() }} />
        ) : null}
      </DialogRoot>
    </main>
  )
}
