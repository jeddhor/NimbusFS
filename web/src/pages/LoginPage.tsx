import * as React from "react"
import { motion } from "framer-motion"
import { FolderLock } from "lucide-react"
import { useAuth } from "@/auth/AuthContext"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { SSHLoginPanel } from "@/components/SSHLoginPanel"
import { api, ApiError } from "@/lib/api"

type AuthMethods = { pam: boolean; sshKeys: boolean; proxyAuth: boolean }

export function LoginPage() {
  const { login } = useAuth()
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [remember, setRemember] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [submitting, setSubmitting] = React.useState(false)
  const [mode, setMode] = React.useState<"password" | "ssh">("password")
  const [methods, setMethods] = React.useState<AuthMethods | null>(null)

  React.useEffect(() => {
    api
      .authMethods()
      .then(setMethods)
      .catch(() => setMethods({ pam: false, sshKeys: false, proxyAuth: false }))
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(username, password, remember)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login failed")
    } finally {
      setSubmitting(false)
    }
  }

  // If proxy auth is the only configured method, a visible login form would
  // never work — the reverse proxy is supposed to authenticate the request
  // before it reaches nimbusfs. Seeing this page at all means the proxy
  // isn't setting X-Remote-User, which is worth saying explicitly.
  const onlyProxyAuth = methods && methods.proxyAuth && !methods.pam && !methods.sshKeys

  return (
    <div className="flex h-full w-full items-center justify-center bg-background p-4">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: "easeOut" }}
        className="glass w-full max-w-sm rounded-2xl p-8 shadow-2xl"
      >
        <div className="mb-6 flex flex-col items-center gap-2 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-accent/15 text-accent">
            <FolderLock size={24} />
          </div>
          <h1 className="text-lg font-semibold text-foreground">NimbusFS</h1>
          <p className="text-sm text-muted">Sign in with your Linux account</p>
        </div>

        {onlyProxyAuth ? (
          <p className="text-sm text-muted">
            This server is configured for reverse-proxy authentication, but no <code>X-Remote-User</code> header was
            received. Check that you're accessing nimbusfs through the proxy, and that the proxy is configured to
            set that header.
          </p>
        ) : (
          <div className="flex flex-col gap-3">
            <Input
              placeholder="Username"
              autoFocus
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />

            {mode === "password" ? (
              <form onSubmit={handleSubmit} className="flex flex-col gap-3">
                <Input
                  placeholder="Password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <label className="flex items-center gap-2 text-sm text-muted">
                  <input
                    type="checkbox"
                    checked={remember}
                    onChange={(e) => setRemember(e.target.checked)}
                    className="h-3.5 w-3.5 accent-accent"
                  />
                  Remember me
                </label>

                {error && <p className="text-sm text-danger">{error}</p>}

                <Button type="submit" disabled={submitting} className="mt-1 w-full">
                  {submitting ? "Signing in..." : "Sign in"}
                </Button>
              </form>
            ) : (
              <SSHLoginPanel username={username} />
            )}

            {methods?.sshKeys && (
              <button
                type="button"
                onClick={() => {
                  setError(null)
                  setMode(mode === "password" ? "ssh" : "password")
                }}
                className="text-xs text-muted underline-offset-2 hover:text-foreground hover:underline"
              >
                {mode === "password" ? "Use an SSH key instead" : "Use a password instead"}
              </button>
            )}
          </div>
        )}
      </motion.div>
    </div>
  )
}
