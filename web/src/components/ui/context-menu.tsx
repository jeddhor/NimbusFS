import * as React from "react"
import { createPortal } from "react-dom"
import { motion } from "framer-motion"
import { cn } from "@/lib/utils"

export interface ContextMenuItem {
  label: string
  icon?: React.ReactNode
  onClick: () => void
  danger?: boolean
  disabled?: boolean
}
export type ContextMenuEntry = ContextMenuItem | { separator: true }

export function ContextMenu({
  x,
  y,
  items,
  onClose,
}: {
  x: number
  y: number
  items: ContextMenuEntry[]
  onClose: () => void
}) {
  const ref = React.useRef<HTMLDivElement>(null)
  const [pos, setPos] = React.useState({ x, y, ready: false })

  React.useEffect(() => {
    function handlePointerDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("mousedown", handlePointerDown)
    window.addEventListener("keydown", handleKey)
    window.addEventListener("scroll", onClose, true)
    window.addEventListener("resize", onClose)
    return () => {
      window.removeEventListener("mousedown", handlePointerDown)
      window.removeEventListener("keydown", handleKey)
      window.removeEventListener("scroll", onClose, true)
      window.removeEventListener("resize", onClose)
    }
  }, [onClose])

  React.useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const { innerWidth, innerHeight } = window
    const rect = el.getBoundingClientRect()
    const nx = x + rect.width > innerWidth ? Math.max(4, innerWidth - rect.width - 4) : x
    const ny = y + rect.height > innerHeight ? Math.max(4, innerHeight - rect.height - 4) : y
    setPos({ x: nx, y: ny, ready: true })
  }, [x, y])

  if (typeof document === "undefined") return null

  return createPortal(
    <motion.div
      ref={ref}
      initial={{ opacity: 0, scale: 0.96 }}
      animate={{ opacity: pos.ready ? 1 : 0, scale: pos.ready ? 1 : 0.96 }}
      transition={{ duration: 0.12 }}
      className="glass fixed z-[60] min-w-[180px] rounded-xl p-1 shadow-2xl"
      style={{ left: pos.x, top: pos.y }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item, i) =>
        "separator" in item ? (
          <div key={i} className="my-1 h-px bg-border" />
        ) : (
          <button
            key={i}
            disabled={item.disabled}
            onClick={() => {
              item.onClick()
              onClose()
            }}
            className={cn(
              "flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-left text-sm transition-colors",
              item.disabled
                ? "cursor-not-allowed text-muted/50"
                : item.danger
                  ? "text-danger hover:bg-danger/10"
                  : "text-foreground hover:bg-white/10",
            )}
          >
            {item.icon}
            {item.label}
          </button>
        ),
      )}
    </motion.div>,
    document.body,
  )
}
