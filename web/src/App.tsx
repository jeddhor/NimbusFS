import { AuthProvider, useAuth } from "@/auth/AuthContext"
import { LoginPage } from "@/pages/LoginPage"
import { BrowserPage } from "@/pages/BrowserPage"

function Gate() {
  const { username, loading } = useAuth()

  if (loading) {
    return <div className="flex h-full w-full items-center justify-center text-sm text-muted">Loading...</div>
  }

  return username ? <BrowserPage /> : <LoginPage />
}

export default function App() {
  return (
    <AuthProvider>
      <Gate />
    </AuthProvider>
  )
}
