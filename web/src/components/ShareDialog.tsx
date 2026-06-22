import * as React from "react"
import { Check, Copy } from "lucide-react"
import { Dialog } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, ApiError, type ShareMode } from "@/lib/api"

export function ShareDialog({
  open,
  path,
  onClose,
}: {
  open: boolean
  path: string | null
  onClose: () => void
}) {
  const [mode, setMode] = React.useState<ShareMode>("both")
  const [password, setPassword] = React.useState("")
  const [expiresInHours, setExpiresInHours] = React.useState(0)
  const [error, setError] = React.useState<string | null>(null)
  const [submitting, setSubmitting] = React.useState(false)
  const [url, setUrl] = React.useState<string | null>(null)
  const [copied, setCopied] = React.useState(false)

  React.useEffect(() => {
    if (open) {
      setMode("both")
      setPassword("")
      setExpiresInHours(0)
      setError(null)
      setUrl(null)
      setCopied(false)
    }
  }, [open])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!path) return
    setSubmitting(true)
    setError(null)
    try {
      const share = await api.createShare({ path, mode, password: password || undefined, expiresInHours })
      setUrl(new URL(share.url, window.location.origin).toString())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create share")
    } finally {
      setSubmitting(false)
    }
  }

  function copyLink() {
    if (!url) return
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Dialog open={open} onClose={onClose} title="Share">
      {url ? (
        <div className="flex flex-col gap-3">
          <p className="text-sm text-muted">Anyone with this link can access it{password && " (with the password)"}.</p>
          <div className="flex gap-2">
            <Input readOnly value={url} onFocus={(e) => e.target.select()} />
            <Button type="button" size="icon" variant="secondary" onClick={copyLink}>
              {copied ? <Check size={14} /> : <Copy size={14} />}
            </Button>
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={onClose} className="self-end">
            Done
          </Button>
        </div>
      ) : (
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <label className="text-xs text-muted">Access</label>
            <div className="flex gap-2">
              {(["both", "view_only", "download_only"] as ShareMode[]).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setMode(m)}
                  className={`flex-1 rounded-lg border px-2 py-1.5 text-xs transition-colors ${
                    mode === m ? "border-accent bg-accent/15 text-foreground" : "border-border text-muted hover:bg-white/5"
                  }`}
                >
                  {m === "both" ? "View & download" : m === "view_only" ? "View only" : "Download only"}
                </button>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-xs text-muted">Expiration</label>
            <select
              value={expiresInHours}
              onChange={(e) => setExpiresInHours(Number(e.target.value))}
              className="h-9 rounded-lg border border-border bg-white/5 px-3 text-sm text-foreground outline-none"
            >
              <option value={0}>Never</option>
              <option value={1}>1 hour</option>
              <option value={24}>1 day</option>
              <option value={24 * 7}>7 days</option>
              <option value={24 * 30}>30 days</option>
            </select>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-xs text-muted">Password (optional)</label>
            <Input
              type="password"
              placeholder="Leave blank for no password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? "Creating..." : "Create link"}
          </Button>
        </form>
      )}
    </Dialog>
  )
}
