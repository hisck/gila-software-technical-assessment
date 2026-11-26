import type { ReactNode } from "react"
import { MarketLayout } from "../../App"

export default function Layout({ children }: { children: ReactNode }) {
  return <MarketLayout>{children}</MarketLayout>
}
