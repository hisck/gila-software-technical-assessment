import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

export function Badge({ className, ...props }: ComponentProps<"span">) {
  return (
    <span data-slot="badge" className={cn("inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold", className)} {...props} />
  )
}
