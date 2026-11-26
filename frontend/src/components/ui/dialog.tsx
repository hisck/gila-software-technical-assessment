"use client"

import * as DialogPrimitive from "@radix-ui/react-dialog"
import { X } from "lucide-react"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"
import { Button } from "./button"

export function DialogRoot(props: ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root {...props} />
}

export function DialogTrigger(props: ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

export function DialogClose(props: ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

type DialogPanelProps = ComponentProps<typeof DialogPrimitive.Content> & {
  title: string
  description: string
  drawer?: boolean
}

export function DialogPanel({ title, description, drawer, children, className, ...props }: DialogPanelProps) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay data-slot="dialog-overlay" className={cn("dialog-overlay", drawer && "drawer-overlay")} />
      <DialogPrimitive.Content data-slot="dialog-content" className={cn("dialog-panel", drawer && "drawer-panel", className)} {...props}>
        <header data-slot="dialog-header" className="dialog-heading">
          <div>
            <DialogPrimitive.Title data-slot="dialog-title">{title}</DialogPrimitive.Title>
            <DialogPrimitive.Description data-slot="dialog-description">{description}</DialogPrimitive.Description>
          </div>
          <DialogClose asChild>
            <Button variant="ghost" size="icon" aria-label="Close dialog"><X /></Button>
          </DialogClose>
        </header>
        {children}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  )
}
