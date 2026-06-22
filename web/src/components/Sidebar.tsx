import { LogOut, FolderOpen } from "lucide-react"
import { useAuth } from "@/auth/AuthContext"
import { Button } from "@/components/ui/button"
import { Logo, Wordmark } from "@/components/Logo"

export function Sidebar({ onNavigateRoot }: { onNavigateRoot: () => void }) {
  const { username, logout } = useAuth()

  return (
    <aside className="glass flex h-full w-60 flex-shrink-0 flex-col rounded-2xl p-4">
      <div className="mb-6 flex items-center gap-2 px-1">
        <Logo size={24} />
        <Wordmark className="text-lg" />
      </div>

      <button
        onClick={onNavigateRoot}
        className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm text-foreground transition-colors hover:bg-white/5"
      >
        <FolderOpen size={16} />
        All Files
      </button>

      <div className="mt-auto flex flex-col gap-2 border-t border-border pt-4">
        <p className="truncate px-1 text-xs text-muted">Signed in as {username}</p>
        <Button variant="secondary" size="sm" onClick={() => logout()} className="justify-start">
          <LogOut size={14} />
          Sign out
        </Button>
      </div>
    </aside>
  )
}
