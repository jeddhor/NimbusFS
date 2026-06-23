import * as React from "react"
import { api } from "@/lib/api"

interface AuthState {
  username: string | null
  loading: boolean
  login: (username: string, password: string, remember: boolean) => Promise<void>
  logout: () => Promise<void>
  setUsername: (username: string) => void
}

const AuthContext = React.createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [username, setUsername] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    api
      .me()
      .then((r) => setUsername(r.username))
      .catch(() => setUsername(null))
      .finally(() => setLoading(false))
  }, [])

  const login = React.useCallback(async (u: string, p: string, remember: boolean) => {
    const res = await api.login(u, p, remember)
    setUsername(res.username)
  }, [])

  const logout = React.useCallback(async () => {
    try {
      await api.logout()
    } finally {
      // Always drop client-side auth state, even if the request failed
      // (e.g. a stale CSRF cookie) — staying "logged in" in the UI while
      // the server already considers the cookie invalid is worse than a
      // local-only sign-out.
      setUsername(null)
    }
  }, [])

  return (
    <AuthContext.Provider value={{ username, loading, login, logout, setUsername }}>{children}</AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = React.useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
