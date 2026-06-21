import * as React from "react"
import { Dialog } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"

export function InputDialog({
  open,
  title,
  label,
  defaultValue = "",
  confirmLabel = "Confirm",
  onClose,
  onConfirm,
}: {
  open: boolean
  title: string
  label: string
  defaultValue?: string
  confirmLabel?: string
  onClose: () => void
  onConfirm: (value: string) => void
}) {
  const [value, setValue] = React.useState(defaultValue)

  React.useEffect(() => {
    if (open) setValue(defaultValue)
  }, [open, defaultValue])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (value.trim()) onConfirm(value.trim())
  }

  return (
    <Dialog open={open} onClose={onClose} title={title}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <label className="text-xs text-muted">{label}</label>
        <Input autoFocus value={value} onChange={(e) => setValue(e.target.value)} />
        <div className="mt-2 flex justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" size="sm">
            {confirmLabel}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
