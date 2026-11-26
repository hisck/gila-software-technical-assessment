import type { Metadata } from "next"
import { OrderHistory } from "../../../components/OrderHistory"

export const metadata: Metadata = { title: "Orders" }

export default function Page() {
  return <OrderHistory />
}
