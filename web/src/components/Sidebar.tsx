import { LogOut } from "lucide-react"
import { useAuth } from "@/auth/AuthContext"
import { Button } from "@/components/ui/button"
import { Logo, Wordmark } from "@/components/Logo"
import { InfoPanel } from "@/components/InfoPanel"
import type { FileEntry } from "@/lib/api"

export function Sidebar({
  selectedEntries,
  fileTypeNames,
}: {
  selectedEntries: FileEntry[]
  fileTypeNames: Record<string, string>
}) {
  const { username, logout } = useAuth()

  return (
    <aside className="glass flex h-full w-60 flex-shrink-0 flex-col rounded-2xl p-4">
      <div className="mb-6 flex items-center gap-2 px-1">
        <Logo size={32} />
        <Wordmark className="text-xl" />
      </div>

      <InfoPanel entries={selectedEntries} fileTypeNames={fileTypeNames} />

      <div className="flex flex-col gap-2 border-t border-border pt-4">
        <p className="truncate px-1 text-xs text-muted">Signed in as {username}</p>
        <Button variant="secondary" size="sm" onClick={() => logout()} className="justify-start">
          <LogOut size={14} />
          Sign out
        </Button>
      </div>
    </aside>
  )
}
