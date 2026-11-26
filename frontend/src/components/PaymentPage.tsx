"use client"

import { ArrowLeft, Check, CircleAlert, Clock3, CreditCard, ShieldCheck } from "lucide-react"
import Link from "next/link"
import { useEffect, useRef, useState } from "react"
import { api, APIError, type PaymentOutcome } from "../api"
import { orderTotal, statusLabels, useOrderStatus } from "../checkout"
import { Badge } from "./ui/badge"
import { Button } from "./ui"

type Handoff = { id: string; choice?: PaymentOutcome }
const paymentKey = "fieldwork.payment.v1"
const orderID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function saveHandoff(handoff: Handoff) {
  try {
    sessionStorage.setItem(paymentKey, JSON.stringify(handoff))
  } catch {
    return
  }
}

function readHandoff(): Handoff | null {
  const id = window.location.hash.slice(1)
  if (id) {
    window.history.replaceState(window.history.state, "", window.location.pathname)
    if (!orderID.test(id)) return null
    const handoff = { id }
    saveHandoff(handoff)
    return handoff
  }
  try {
    const handoff = JSON.parse(sessionStorage.getItem(paymentKey) ?? "null") as Handoff | null
    if (handoff && orderID.test(handoff.id)) return handoff
  } catch {
    return null
  }
  return null
}

export function PaymentPage() {
  const [handoff, setHandoff] = useState<Handoff | null | undefined>(undefined)
  const [busy, setBusy] = useState(false)
  const [accepted, setAccepted] = useState(false)
  const [failure, setFailure] = useState("")
  const sending = useRef(false)
  const { order, error, refresh } = useOrderStatus(handoff?.id)

  useEffect(() => {
    setHandoff(readHandoff())
  }, [])

  async function choose(choice: PaymentOutcome) {
    if (sending.current || !handoff || accepted || order?.status !== "awaiting_payment") return
    sending.current = true
    setBusy(true)
    setFailure("")
    const next = { ...handoff, choice }
    setHandoff(next)
    saveHandoff(next)
    try {
      await api.pay(handoff.id, choice)
      setAccepted(true)
    } catch (error) {
      if (error instanceof APIError && error.status === 409) {
        setAccepted(true)
        setFailure("A payment choice was already recorded for this order.")
      } else {
        setFailure(error instanceof Error ? error.message : "Payment is temporarily unavailable.")
      }
    } finally {
      refresh()
      sending.current = false
      setBusy(false)
    }
  }

  const status = accepted && order?.status === "awaiting_payment" ? "pending_inventory" : order?.status
  const Icon = status === "paid" ? Check : status === "rejected" ? CircleAlert : Clock3
  const showChoices = status === "awaiting_payment"
  const choice = handoff?.choice
  const alert = (order?.status === "awaiting_payment" && failure) || error

  return (
    <main className="payment-page">
      <div className="payment-frame">
        <header className="payment-brand">
          <span className="payment-brand-mark"><CreditCard /></span>
          <strong>Fieldwork Pay</strong>
          <Badge className="payment-simulation"><ShieldCheck /> Simulation</Badge>
        </header>
        <section className="payment-card" aria-labelledby="payment-title">
          <header>
            <h1 id="payment-title">Simulated payment</h1>
            {showChoices && <p>Choose an outcome for this order. No payment details are needed.</p>}
          </header>

          {handoff === null && (
            <p className="feedback">Open a pending order from Fieldwork Market to start payment.</p>
          )}
          {handoff !== null && !order && !error && <p role="status" className="muted">Loading payment…</p>}

          {order && status && (
            <>
              <div className="payment-amount">
                <span>{order.total === undefined ? "Estimated total" : "Order total"}</span>
                <strong>{orderTotal(order)}</strong>
                <small>Order #{order.number}</small>
              </div>
              <div className={`order-status ${status}`} role="status" aria-live="polite" aria-atomic="true">
                <Icon />
                <div>
                  <h2>{statusLabels[status]}</h2>
                  {order.reason && <p>{order.reason}</p>}
                </div>
              </div>
            </>
          )}

          {alert && <p role="alert" className="feedback feedback-error">{alert}</p>}

          {showChoices && (
            <div className="payment-actions">
              {choice && !busy ? (
                <Button onClick={() => void choose(choice)}>Retry {choice === "approve" ? "approval" : "decline"}</Button>
              ) : (
                <>
                  <Button disabled={busy} onClick={() => void choose("approve")}><Check /> Approve payment</Button>
                  <Button variant="outline" disabled={busy} onClick={() => void choose("decline")}><CircleAlert /> Decline payment</Button>
                </>
              )}
              {busy && <p role="status" className="muted">Submitting payment…</p>}
            </div>
          )}

          <Button variant="ghost" className="payment-back" asChild>
            <Link href="/orders/"><ArrowLeft /> View orders in Fieldwork Market</Link>
          </Button>
        </section>
      </div>
    </main>
  )
}
