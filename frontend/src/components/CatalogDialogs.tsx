import { Download, FileText, TriangleAlert, Upload } from "lucide-react"
import { useEffect, useState } from "react"
import type { FormEvent } from "react"
import { api, APIError, type ImportReport, type Product, type ProductFields } from "../api"
import { Button, DialogClose, DialogPanel } from "./ui"

type DialogContext = {
  busy: boolean
  setBusy: (value: boolean) => void
  restoreFocus: (event: Event) => void
}

export function ProductEditor({ id, busy, setBusy, restoreFocus, onSaved }: DialogContext & {
  id?: string
  onSaved: (product: Product) => void
}) {
  const [product, setProduct] = useState<Product>()
  const [error, setError] = useState("")
  const [loadError, setLoadError] = useState("")
  const [revision, setRevision] = useState(0)

  useEffect(() => {
    if (!id) return
    const controller = new AbortController()
    setProduct(undefined)
    setLoadError("")
    void api.product(id, controller.signal)
      .then((loaded) => { if (!controller.signal.aborted) setProduct(loaded) })
      .catch((failure: Error) => { if (!controller.signal.aborted) setLoadError(failure.message) })
    return () => controller.abort()
  }, [id, revision])

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    const form = new FormData(event.currentTarget)
    const value = (field: string) => String(form.get(field) ?? "").trim()
    const fields: ProductFields = {
      name: value("name"),
      sku: value("sku"),
      description: value("description"),
      category: value("category"),
      price: value("price"),
      stock: Number(value("stock")),
      weight_kg: value("weight_kg"),
    }
    if (!fields.name || !fields.sku || !fields.category) {
      setError("Enter a name, SKU, and category.")
      return
    }

    setBusy(true)
    try {
      onSaved(await api.saveProduct(fields, product))
    } catch (failure) {
      if (failure instanceof APIError && failure.status === 412) {
        setError("Inventory changed while you were editing. Reload the product before saving again.")
      } else {
        setError(failure instanceof Error ? failure.message : "The product could not be saved.")
      }
    } finally {
      setBusy(false)
    }
  }

  function reload() {
    setError("")
    setRevision((value) => value + 1)
  }

  return (
    <DialogPanel title={id ? "Edit product" : "Add product"} description="Product details and available inventory." onCloseAutoFocus={restoreFocus}>
      {id && !product && (
        <div className="dialog-body">
          {loadError ? (
            <>
              <p className="feedback feedback-error" role="alert">{loadError}</p>
              <Button variant="outline" onClick={reload}>Retry loading product</Button>
            </>
          ) : <p className="muted" role="status">Loading product…</p>}
        </div>
      )}
      {(!id || product) && (
        <form onSubmit={(event) => void save(event)}>
          <fieldset className="dialog-body editor-fields" disabled={busy}>
            <label>Name<input name="name" required maxLength={160} defaultValue={product?.name} /></label>
            <label>SKU<input name="sku" required maxLength={64} pattern={"[A-Za-z0-9][A-Za-z0-9._\\-]{0,63}"} title="Use letters, numbers, periods, underscores, and hyphens. Start with a letter or number." defaultValue={product?.sku} /></label>
            <label>Category<input name="category" required maxLength={100} defaultValue={product?.category} /></label>
            <label>Price (USD)<input name="price" required inputMode="decimal" pattern={"[0-9]{1,9}(\\.[0-9]{1,2})?"} title="Enter a decimal amount with up to two fractional digits." placeholder="0.00" defaultValue={product?.price} /></label>
            <label>Available stock<input type="number" name="stock" required min={0} max={1000000} step={1} defaultValue={product?.stock ?? 0} /></label>
            <label>Weight (kg)<input name="weight_kg" required inputMode="decimal" pattern={"[0-9]{1,9}(\\.[0-9]{1,3})?"} title="Enter a decimal weight with up to three fractional digits." placeholder="0.000" defaultValue={product?.weight_kg} /></label>
            <label className="full-field">Description <span className="muted">(optional)</span><textarea name="description" maxLength={4000} rows={3} defaultValue={product?.description} /></label>
            {error && <p className="feedback feedback-error full-field" role="alert">{error}</p>}
          </fieldset>
          <footer className="dialog-footer">
            {error.startsWith("Inventory changed") && <Button variant="outline" onClick={reload}>Reload product</Button>}
            <DialogClose asChild><Button variant="ghost" disabled={busy}>Cancel</Button></DialogClose>
            <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Save product"}</Button>
          </footer>
        </form>
      )}
    </DialogPanel>
  )
}

export function DeleteProduct({ product, busy, setBusy, restoreFocus, onDeleted }: DialogContext & {
  product: Product
  onDeleted: () => void
}) {
  const [error, setError] = useState("")

  async function remove() {
    setBusy(true)
    setError("")
    try {
      await api.deleteProduct(product)
      onDeleted()
    } catch (failure) {
      if (failure instanceof APIError && failure.status === 412) {
        setError("This product changed. Close this dialog and refresh the product list before trying again.")
      } else {
        setError(failure instanceof Error ? failure.message : "The product could not be deleted.")
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <DialogPanel title="Delete product?" description={`${product.name} will be removed from the catalog.`} onCloseAutoFocus={restoreFocus}
      onOpenAutoFocus={(event) => {
        event.preventDefault()
        document.getElementById("cancel-delete")?.focus()
      }}>
      {error && <div className="dialog-body"><p className="feedback feedback-error" role="alert">{error}</p></div>}
      <footer className="dialog-footer">
        <DialogClose asChild><Button id="cancel-delete" variant="outline" disabled={busy}>Cancel</Button></DialogClose>
        <Button variant="danger" disabled={busy} onClick={() => void remove()}>{busy ? "Deleting…" : "Delete product"}</Button>
      </footer>
    </DialogPanel>
  )
}

export function ImportProducts({ busy, setBusy, restoreFocus, onImported }: DialogContext & {
  onImported: (report: ImportReport) => void
}) {
  const [file, setFile] = useState<File | null>(null)
  const [report, setReport] = useState<ImportReport>()
  const [error, setError] = useState("")

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!file) return
    setError("")
    setReport(undefined)
    if (file.size > 1024 * 1024) {
      setError("Choose a CSV file up to 1 MB.")
      return
    }

    setBusy(true)
    try {
      const result = await api.importProducts(file)
      setReport(result)
      onImported(result)
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The file could not be imported.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <DialogPanel title="Import products" description="Upload a CSV to add products. Existing SKUs are preserved." onCloseAutoFocus={restoreFocus}>
      <form onSubmit={(event) => void submit(event)}>
        <div className="dialog-body import-area">
          <label className="file-picker">
            <FileText aria-hidden="true" />
            <span>Product CSV <small>Up to 1 MB</small></span>
            <input required type="file" accept=".csv,text/csv" disabled={busy} aria-label="Product CSV" onChange={(event) => {
              setFile(event.target.files?.[0] ?? null)
              setReport(undefined)
              setError("")
            }} />
          </label>
          <a className="text-link" href="/sample-products.csv" download><Download /> Download sample CSV</a>
          {error && <p className="feedback feedback-error" role="alert">{error}</p>}
          {report && (
            <section aria-label="Import report" className="import-report">
              <p role="status" className="feedback feedback-success">
                {report.imported} imported · {report.skipped} skipped
              </p>
              {!!report.issues?.length && (
                <>
                  <h3><TriangleAlert /> Review skipped rows</h3>
                  <ul className="import-issues">
                    {report.issues.map((issue) => (
                      <li key={issue.line}><strong>Line {issue.line}{issue.sku ? ` · ${issue.sku}` : ""}</strong><span>{issue.message}</span></li>
                    ))}
                  </ul>
                </>
              )}
            </section>
          )}
        </div>
        <footer className="dialog-footer">
          <DialogClose asChild><Button variant="ghost" disabled={busy}>{report ? "Done" : "Cancel"}</Button></DialogClose>
          <Button type="submit" disabled={busy || !file}><Upload />{busy ? "Importing…" : "Import CSV"}</Button>
        </footer>
      </form>
    </DialogPanel>
  )
}
