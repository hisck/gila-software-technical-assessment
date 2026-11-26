import type { Metadata, Viewport } from "next"
import type { ReactNode } from "react"
import "../styles.css"

export const metadata: Metadata = {
  title: { default: "Fieldwork Market", template: "%s | Fieldwork Market" },
  description: "Shop the Fieldwork Market catalog.",
}

export const viewport: Viewport = { themeColor: "#f8f7f2" }

export default function RootLayout({ children }: { children: ReactNode }) {
  return <html lang="en"><body>{children}</body></html>
}
