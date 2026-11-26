import {
  ArrowDown, BookOpen, Coffee, Dumbbell, Gamepad2, Gem, Headphones, House, Mountain,
  Package, PawPrint, Plus, RefreshCw, Search, Shirt, Sparkles, Utensils,
} from "lucide-react"
import { useEffect, useState } from "react"
import type { FormEvent } from "react"
import { api, type Product, type SearchPage } from "../api"
import { usd } from "../checkout"
import { PageControls } from "./PageControls"
import { Button } from "./ui"

const themes = {
  accessories: [Gem, "rose"],
  beauty: [Sparkles, "rose"],
  books: [BookOpen, "stone"],
  clothing: [Shirt, "stone"],
  electronics: [Headphones, "blue"],
  "food & beverage": [Coffee, "ochre"],
  footwear: [Shirt, "stone"],
  games: [Gamepad2, "blue"],
  "home & office": [House, "olive"],
  kitchen: [Utensils, "ochre"],
  outdoors: [Mountain, "olive"],
  pets: [PawPrint, "rose"],
  sports: [Dumbbell, "ochre"],
  stationery: [BookOpen, "stone"],
} as const

function ProductCard({ product, onAdd }: { product: Product; onAdd: (product: Product, button: HTMLButtonElement) => void }) {
  const category = product.category.toLowerCase()
  const [Icon, tone] = Object.hasOwn(themes, category) ? themes[category as keyof typeof themes] : [Package, "stone"]

  return (
    <article className="product-card">
      <div className={`category-art tone-${tone}`} aria-hidden="true"><Icon strokeWidth={1.2} /></div>
      <div className="product-card-body">
        <div className="product-meta"><span>{product.category}</span><span className="sku">{product.sku}</span></div>
        <h3>{product.name}</h3>
        {product.description && <p className="product-description">{product.description}</p>}
        <div className="product-card-footer">
          <div><strong>{usd(product.price)}</strong><span className={product.stock ? "in-stock" : "out-stock"}>{product.stock ? `${product.stock.toLocaleString("en-US")} in stock` : "Out of stock"}</span></div>
          <Button size="small" variant="outline" disabled={product.stock === 0} aria-label={`Add ${product.name} to bag`} onClick={(event) => onAdd(product, event.currentTarget)}>
            <Plus /> Add
          </Button>
        </div>
      </div>
    </article>
  )
}

export function Storefront({ onAdd, orderUpdate }: { onAdd: (product: Product, button: HTMLButtonElement) => void; orderUpdate?: string }) {
  const [searchText, setSearchText] = useState("")
  const [query, setQuery] = useState("")
  const [category, setCategory] = useState("")
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [result, setResult] = useState<SearchPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError("")

    void api.search(query, category, page, controller.signal)
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
        setResult(null)
        setError(failure.message)
        setLoading(false)
      })

    return () => controller.abort()
  }, [query, category, page, revision, orderUpdate])

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setQuery(searchText.trim())
    setRevision((current) => current + 1)
  }

  function clearFilters() {
    setSearchText("")
    setQuery("")
    setCategory("")
    setPage(1)
  }

  const categories = result?.categories ?? []

  return (
    <main id="main-content" className="page-shell storefront" tabIndex={-1}>
      <section className="store-hero" aria-labelledby="store-title">
        <div className="hero-copy">
          <h1 id="store-title">Good finds for<br />everyday life.</h1>
          <Button asChild className="hero-action"><a href="#products">Explore the catalog <ArrowDown /></a></Button>
        </div>
      </section>

      <section className="products-section" id="products" aria-labelledby="products-title">
        <div className="section-heading">
          <h2 id="products-title">Browse products</h2>
          <Button size="icon" variant="outline" aria-label="Refresh products" title="Refresh products" disabled={loading} onClick={() => setRevision((current) => current + 1)}>
            <RefreshCw />
          </Button>
        </div>
        <div className="store-controls">
          <form role="search" className="search-form" onSubmit={search}>
            <label className="search-field">
              <Search aria-hidden="true" />
              <span className="sr-only">Search products</span>
              <input type="search" value={searchText} maxLength={120} onChange={(event) => setSearchText(event.target.value)} placeholder="Search products, SKUs, and more" />
            </label>
            <Button type="submit">Search</Button>
          </form>
          <label className="category-filter">
            <span className="sr-only">Product category</span>
            <select value={category} onChange={(event) => { setCategory(event.target.value); setPage(1) }}>
              <option value="">All categories</option>
              {category && !categories.includes(category) && <option value={category}>{category}</option>}
              {categories.map((name) => <option value={name} key={name}>{name}</option>)}
            </select>
          </label>
        </div>

        <p className="results-summary" role="status">
          {loading ? "Finding products…" : result ? `${result.total.toLocaleString("en-US")} ${result.total === 1 ? "product" : "products"}${query ? ` matching “${query}”` : ""}` : ""}
        </p>

        {error && (
          <div className="empty-state">
            <p className="feedback feedback-error" role="alert">{error}</p>
            <Button variant="outline" onClick={() => setRevision((current) => current + 1)}>Retry search</Button>
          </div>
        )}

        {loading && !result && (
          <div className="product-grid" aria-hidden="true">
            {Array.from({ length: 4 }, (_, index) => <div className="product-skeleton" key={index} />)}
          </div>
        )}

        {result && (
          <div aria-busy={loading}>
            {result.items.length ? (
              <div className="product-grid">
                {result.items.map((product) => <ProductCard key={product.id} product={product} onAdd={onAdd} />)}
              </div>
            ) : !loading && (
              <div className="empty-state">
                <Package />
                <h3>{query || category ? "No matching products" : "The catalog is empty"}</h3>
                <p>{query || category ? "Try a different term or category." : "Add a product or import the example CSV from Manage catalog."}</p>
                {(query || category) && <Button variant="outline" onClick={clearFilters}>Clear filters</Button>}
              </div>
            )}
            <PageControls page={result.page} limit={result.limit} total={result.total} onChange={setPage} />
          </div>
        )}
      </section>
    </main>
  )
}
