import { BrowserRouter, Routes, Route } from "react-router-dom"
import { AuthProvider, useAuth } from "@/auth/AuthContext"
import { LoginPage } from "@/pages/LoginPage"
import { BrowserPage } from "@/pages/BrowserPage"
import { ShareViewPage } from "@/pages/ShareViewPage"

function Gate() {
  const { username, loading } = useAuth()

  if (loading) {
    return <div className="flex h-full w-full items-center justify-center text-sm text-muted">Loading...</div>
  }

  return username ? <BrowserPage /> : <LoginPage />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/share/:token" element={<ShareViewPage />} />
        <Route
          path="*"
          element={
            <AuthProvider>
              <Gate />
            </AuthProvider>
          }
        />
      </Routes>
    </BrowserRouter>
  )
}
