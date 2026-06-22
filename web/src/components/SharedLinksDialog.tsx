import * as React from "react"
import { Trash2 } from "lucide-react"
import { Dialog } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { formatDate } from "@/lib/utils"
import { api, ApiError, type Share } from "@/lib/api"

export function SharedLinksDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [shares, setShares] = React.useState<Share[]>([])
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(true)

  const refresh = React.useCallback(() => {
    setLoading(true)
    api
      .listShares()
      .then((res) => setShares(res.shares))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Could not load shares"))
      .finally(() => setLoading(false))
  }, [])

  React.useEffect(() => {
    if (open) refresh()
  }, [open, refresh])

  async function revoke(token: string) {
    try {
      await api.revokeShare(token)
      setShares((prev) => prev.filter((s) => s.token !== token))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not revoke share")
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title="Shared Links" className="max-w-lg">
      {error && <p className="mb-2 text-sm text-danger">{error}</p>}
      {loading ? (
        <p className="text-sm text-muted">Loading...</p>
      ) : shares.length === 0 ? (
        <p className="text-sm text-muted">You haven't shared anything yet.</p>
      ) : (
        <div className="flex max-h-96 flex-col gap-1 overflow-auto">
          {shares.map((s) => (
            <div key={s.token} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-white/5">
              <div className="flex-1 overflow-hidden">
                <p className="truncate text-sm text-foreground">{s.name}</p>
                <p className="truncate text-xs text-muted">
                  {s.mode === "both" ? "View & download" : s.mode === "view_only" ? "View only" : "Download only"}
                  {s.hasPassword && " · password protected"}
                  {s.expiresAt ? ` · expires ${formatDate(s.expiresAt)}` : " · never expires"}
                </p>
              </div>
              <Button size="icon" variant="ghost" onClick={() => revoke(s.token)}>
                <Trash2 size={14} />
              </Button>
            </div>
          ))}
        </div>
      )}
    </Dialog>
  )
}
