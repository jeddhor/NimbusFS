import * as React from "react"
import { Terminal, Loader2 } from "lucide-react"
import { useAuth } from "@/auth/AuthContext"
import { Button } from "@/components/ui/button"
import { api, ApiError } from "@/lib/api"

const POLL_INTERVAL_MS = 2000

export function SSHLoginPanel({ username }: { username: string }) {
  const { setUsername } = useAuth()
  const [error, setError] = React.useState<string | null>(null)
  const [starting, setStarting] = React.useState(false)
  const [challenge, setChallenge] = React.useState<{ code: string; pollToken: string; expiresAt: number } | null>(
    null,
  )

  React.useEffect(() => {
    if (!challenge) return
    const interval = setInterval(async () => {
      if (Date.now() > challenge.expiresAt) {
        setError("Code expired. Start again.")
        setChallenge(null)
        return
      }
      try {
        const res = await api.sshPoll(challenge.pollToken)
        if (res.status === "approved" && res.username) {
          setUsername(res.username)
        }
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Polling failed")
        setChallenge(null)
      }
    }, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [challenge, setUsername])

  async function start() {
    if (!username) {
      setError("Enter your username first")
      return
    }
    setError(null)
    setStarting(true)
    try {
      const res = await api.sshStart(username)
      setChallenge({ code: res.code, pollToken: res.pollToken, expiresAt: Date.now() + res.expiresIn * 1000 })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start SSH login")
    } finally {
      setStarting(false)
    }
  }

  const origin = typeof window !== "undefined" ? window.location.origin : ""

  if (!challenge) {
    return (
      <div className="flex flex-col gap-3">
        <Button type="button" variant="secondary" onClick={start} disabled={starting} className="w-full">
          {starting ? <Loader2 size={14} className="animate-spin" /> : <Terminal size={14} />}
          Sign in with SSH key
        </Button>
        {error && <p className="text-sm text-danger">{error}</p>}
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center gap-3 text-center">
      <p className="text-sm text-muted">Run this on the machine holding your SSH key:</p>
      <code className="w-full rounded-lg bg-white/5 px-3 py-2 text-xs text-foreground">
        nimbusfs ssh-login --server {origin} --code {challenge.code}
      </code>
      <p className="flex items-center gap-2 text-xs text-muted">
        <Loader2 size={12} className="animate-spin" />
        Waiting for confirmation...
      </p>
      {error && <p className="text-sm text-danger">{error}</p>}
      <Button type="button" variant="ghost" size="sm" onClick={() => setChallenge(null)}>
        Cancel
      </Button>
    </div>
  )
}
