import type { Metadata } from "next"
import { PaymentPage } from "../../components/PaymentPage"

export const metadata: Metadata = {
  title: { absolute: "Simulated checkout | Fieldwork Pay" },
  robots: { index: false, follow: false },
}

export default function Page() {
  return <PaymentPage />
}
