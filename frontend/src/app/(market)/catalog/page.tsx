import type { Metadata } from "next"
import { CatalogAdmin } from "../../../components/CatalogAdmin"

export const metadata: Metadata = { title: "Manage catalog" }

export default function Page() {
  return <CatalogAdmin />
}
