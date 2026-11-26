import { ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "./ui"

export function PageControls({ page, limit, total, onChange, label = "Product pages" }: {
  page: number
  limit: number
  total: number
  onChange: (page: number) => void
  label?: string
}) {
  const lastPage = Math.max(1, Math.ceil(total / limit))
  if (lastPage === 1) return null

  return (
    <nav className="pagination" aria-label={label}>
      <Button variant="outline" size="small" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        <ChevronLeft /> Previous
      </Button>
      <span aria-live="polite">Page {page} of {lastPage}</span>
      <Button variant="outline" size="small" disabled={page >= lastPage} onClick={() => onChange(page + 1)}>
        Next <ChevronRight />
      </Button>
    </nav>
  )
}
