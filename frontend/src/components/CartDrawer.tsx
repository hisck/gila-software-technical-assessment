import { ArrowRight, Check, CircleAlert, Clock3, ExternalLink, Minus, Plus, ShoppingBag, Trash2 } from "lucide-react"
import Link from "next/link"
import { orderFinished, orderTotal, paymentHref, statusLabels, usd, type useCart } from "../checkout"
import { Button, DialogClose, DialogPanel } from "./ui"

export function CartDrawer({ cart, addedMessage, onCloseAutoFocus }: {
  cart: ReturnType<typeof useCart>
  addedMessage?: string
  onCloseAutoFocus: (event: Event) => void
}) {
  const { attempt, lines, order } = cart
  const finished = orderFinished(order)

  async function launchPayment() {
    const tab = window.open("about:blank", "_blank")
    if (tab) tab.opener = null
    const id = await cart.submit()
    if (!id) {
      tab?.close()
      return
    }
    if (tab && !tab.closed) tab.location.replace(new URL(paymentHref(id), window.location.origin).href)
  }

  return (
    <DialogPanel
      title="Your bag"
      description={addedMessage ?? "Review your items before opening the payment page."}
      className={addedMessage ? "drawer-confirmed" : undefined}
      onCloseAutoFocus={onCloseAutoFocus}
      drawer
    >
      <div className="drawer-body">
        {attempt?.id && (
          <section className={`order-status ${order?.status ?? ""}`} aria-live="polite" aria-atomic="true">
            {order?.status === "paid" ? <Check /> : order?.status === "rejected" ? <CircleAlert /> : <Clock3 />}
            <div>
              <h3>{order ? statusLabels[order.status] : "Order received"}</h3>
              {order && <p>Order #{order.number}</p>}
              {order?.reason && <p>{order.reason}</p>}
              {order?.total !== undefined && <p className="order-total">{order.status === "paid" ? "Paid" : "Order total"} {orderTotal(order)}</p>}
            </div>
          </section>
        )}

        {order?.status === "paid" && order.items.length > 0 && (
          <ul className="receipt-items">
            {order.items.map((item) => (
              <li key={item.product_id}>
                <span>{item.quantity} × {item.name ?? item.sku ?? "Product"}</span>
                <strong>{usd(item.unit_price)}</strong>
              </li>
            ))}
          </ul>
        )}

        {lines.length > 0 && (
          <ul className="cart-items">
            {lines.map(({ product, quantity }) => (
              <li key={product.id} className="cart-line">
                <div className="cart-line-details">
                  <span className="sku">{product.sku}</span>
                  <strong>{product.name}</strong>
                  <span>{usd(product.price)} each</span>
                </div>
                <div className="cart-line-actions">
                  <div className="quantity-control" aria-label={`${product.name} quantity`}>
                    <Button variant="ghost" size="icon" disabled={!!attempt || quantity <= 1} aria-label={`Decrease quantity for ${product.name}`} onClick={() => cart.changeQuantity(product.id, quantity - 1)}><Minus /></Button>
                    <span aria-live="polite">{quantity}</span>
                    <Button variant="ghost" size="icon" disabled={!!attempt || quantity >= product.stock} aria-label={`Increase quantity for ${product.name}`} onClick={() => cart.changeQuantity(product.id, quantity + 1)}><Plus /></Button>
                  </div>
                  <Button variant="ghost" size="icon" disabled={!!attempt} aria-label={`Remove ${product.name} from bag`} onClick={() => cart.changeQuantity(product.id, 0)}><Trash2 /></Button>
                </div>
              </li>
            ))}
          </ul>
        )}

        {!lines.length && !attempt && (
          <div className="empty-state cart-empty">
            <ShoppingBag />
            <h3>Your bag is empty</h3>
            <p>Browse the storefront and add a product to get started.</p>
            <DialogClose asChild><Button variant="outline">Continue browsing</Button></DialogClose>
          </div>
        )}

        {cart.error && <p role="alert" className="feedback feedback-error">{cart.error}</p>}
        {attempt && !attempt.id && !cart.submitting && (
          <p className="feedback">This checkout has no confirmed response yet. Retrying uses the same order request.</p>
        )}
        {cart.reviewPriorOrder && (
          <div className="feedback">
            <p>Check Orders for a previous checkout before creating another order.</p>
            <DialogClose asChild><Button variant="outline" asChild><Link href="/orders/">View orders</Link></Button></DialogClose>
            <Button variant="ghost" onClick={cart.acknowledgePriorOrder}>I checked my orders</Button>
          </div>
        )}
      </div>

      {(lines.length > 0 || attempt) && (
        <footer className="drawer-footer">
          {lines.length > 0 && <div className="cart-subtotal"><span>Subtotal</span><strong>{usd(cart.total)}</strong></div>}
          {attempt?.id
            ? (
              <>
                {(!order || order.status === "awaiting_payment") && (
                  <Button asChild className="wide">
                    <a href={paymentHref(attempt.id)} target="_blank" rel="noopener noreferrer">Open payment <ExternalLink /></a>
                  </Button>
                )}
                <DialogClose asChild>
                  <Button asChild className="wide" variant="outline"><Link href="/orders/">View orders</Link></Button>
                </DialogClose>
                {finished && <Button variant="ghost" onClick={cart.finishAttempt}>Clear receipt</Button>}
              </>
            )
            : (
              <Button className="wide" disabled={cart.submitting || cart.reviewPriorOrder} onClick={() => void launchPayment()}>
                {cart.submitting ? "Creating order…" : attempt ? "Retry checkout" : "Continue to payment"}
                {!cart.submitting && <ArrowRight />}
              </Button>
            )}
          {addedMessage && lines.length > 0 && (
            <DialogClose asChild><Button variant="ghost">Continue shopping</Button></DialogClose>
          )}
        </footer>
      )}
    </DialogPanel>
  )
}
