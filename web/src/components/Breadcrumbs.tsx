import { Home, ChevronRight } from "lucide-react"

export function Breadcrumbs({ path, onNavigate }: { path: string; onNavigate: (path: string) => void }) {
  const segments = path.split("/").filter(Boolean)

  return (
    <div className="glass flex items-center gap-1 rounded-xl px-3 py-2 text-sm">
      <button
        onClick={() => onNavigate("/")}
        className="flex items-center gap-1 rounded-md px-2 py-1 text-muted transition-colors hover:bg-white/5 hover:text-foreground"
      >
        <Home size={14} />
      </button>
      {segments.map((seg, i) => {
        const segPath = "/" + segments.slice(0, i + 1).join("/")
        return (
          <span key={segPath} className="flex items-center gap-1">
            <ChevronRight size={14} className="text-muted" />
            <button
              onClick={() => onNavigate(segPath)}
              className="rounded-md px-2 py-1 text-foreground transition-colors hover:bg-white/5"
            >
              {seg}
            </button>
          </span>
        )
      })}
    </div>
  )
}
