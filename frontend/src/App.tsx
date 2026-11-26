"use client"

import { ClipboardList, Layers2, Package, ShoppingBag } from "lucide-react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react"
import { useCart } from "./checkout"
import type { Product } from "./api"
import { CartDrawer } from "./components/CartDrawer"
import { Storefront } from "./components/Storefront"
import { Button, DialogRoot, DialogTrigger } from "./components/ui"

const Market = createContext<{ onAdd: (product: Product, button: HTMLButtonElement) => void; orderUpdate?: string }>({ onAdd: () => {} })

export function StorefrontPage() {
  return <Storefront {...useContext(Market)} />
}

export function MarketLayout({ children }: { children: ReactNode }) {
  const route = usePathname().replace(/\/$/, "") || "/"
  const [announcement, setAnnouncement] = useState<{ text: string }>()
  const [bagOpen, setBagOpen] = useState(false)
  const [addedMessage, setAddedMessage] = useState<string>()
  const addButton = useRef<HTMLButtonElement | null>(null)
  const cart = useCart()
  const count = cart.lines.reduce((sum, line) => sum + line.quantity, 0)
  const order = cart.order

  useEffect(() => {
    if (!announcement) return
    const timer = window.setTimeout(() => setAnnouncement(undefined), 4200)
    return () => window.clearTimeout(timer)
  }, [announcement])

  function addToBag(product: Product, button: HTMLButtonElement) {
    const reason = cart.add(product)
    if (reason) {
      setAnnouncement({ text: reason })
      return
    }
    addButton.current = button
    setAnnouncement(undefined)
    setAddedMessage(`${product.name} added to your bag.`)
    setBagOpen(true)
  }

  return (
    <div className="market-layout">
      <a className="skip-link" href="#main-content">Skip to content</a>
      <header className="site-header">
        <div className="header-inner page-shell">
          <Link className="brand" href="/" aria-label="Fieldwork Market home">
            <span className="brand-mark"><Layers2 /></span>
            <span>fieldwork<span className="brand-period">.</span> <small>MARKET</small></span>
          </Link>
          <nav className="main-nav" aria-label="Main navigation">
            <Link href="/" aria-current={route === "/" ? "page" : undefined}>Storefront</Link>
            <Link href="/catalog/" aria-current={route === "/catalog" ? "page" : undefined}><Package /> Manage catalog</Link>
            <Link href="/orders/" aria-current={route === "/orders" ? "page" : undefined}><ClipboardList /> Orders</Link>
          </nav>
          <DialogRoot open={bagOpen} onOpenChange={(open) => {
            setBagOpen(open)
            if (open) setAddedMessage(undefined)
          }}>
            <DialogTrigger asChild>
              <Button variant="outline" className="bag-trigger" aria-label={`Your bag, ${count} ${count === 1 ? "item" : "items"}`}>
                <ShoppingBag /><span className="bag-label">Bag</span><span className="bag-count">{count}</span>
              </Button>
            </DialogTrigger>
            <CartDrawer cart={cart} addedMessage={addedMessage} onCloseAutoFocus={(event) => {
              const button = addButton.current
              addButton.current = null
              if (button?.isConnected) {
                event.preventDefault()
                button.focus({ preventScroll: true })
              }
            }} />
          </DialogRoot>
        </div>
      </header>

      <Market.Provider value={{
        onAdd: addToBag,
        orderUpdate: order ? `${order.id}:${order.status}` : undefined,
      }}>
        {children}
      </Market.Provider>

      <footer className="site-footer">
        <div className="page-shell">
          <span>Fieldwork Market</span>
          <span>USD · Simulated payments</span>
        </div>
      </footer>
      <div className="announcements" role="status" aria-live="polite" aria-atomic="true">
        {announcement && <p>{announcement.text}</p>}
      </div>
    </div>
  )
}
