import * as React from "react"
import { Search, X, Loader2 } from "lucide-react"
import { FileTypeIcon } from "@/lib/fileIcons"
import { formatBytes } from "@/lib/utils"
import { api, ApiError, type FileEntry } from "@/lib/api"

const DEBOUNCE_MS = 200

export function SearchBar({
  onNavigate,
  onOpen,
}: {
  onNavigate: (path: string) => void
  onOpen: (entry: FileEntry) => void
}) {
  const [query, setQuery] = React.useState("")
  const [results, setResults] = React.useState<FileEntry[]>([])
  const [loading, setLoading] = React.useState(false)
  const [indexing, setIndexing] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [open, setOpen] = React.useState(false)
  const containerRef = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    if (!query.trim()) {
      setResults([])
      setOpen(false)
      return
    }
    setLoading(true)
    const handle = setTimeout(() => {
      api
        .search(query)
        .then((res) => {
          setResults(res.entries)
          setIndexing(res.indexing)
          setError(null)
          setOpen(true)
        })
        .catch((err) => setError(err instanceof ApiError ? err.message : "Search failed"))
        .finally(() => setLoading(false))
    }, DEBOUNCE_MS)
    return () => clearTimeout(handle)
  }, [query])

  React.useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [])

  function select(entry: FileEntry) {
    setOpen(false)
    setQuery("")
    if (entry.isDir) onNavigate(entry.path)
    else onOpen(entry)
  }

  return (
    <div ref={containerRef} className="glass relative w-72 rounded-xl">
      <div className="flex items-center gap-2 px-3 py-2">
        {loading ? <Loader2 size={14} className="animate-spin text-muted" /> : <Search size={14} className="text-muted" />}
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => results.length > 0 && setOpen(true)}
          placeholder="Search files..."
          className="w-full bg-transparent text-sm text-foreground outline-none placeholder:text-muted"
        />
        {query && (
          <button onClick={() => setQuery("")} className="text-muted hover:text-foreground">
            <X size={14} />
          </button>
        )}
      </div>

      {open && (
        <div className="glass absolute left-0 top-full z-40 mt-1 max-h-96 w-full overflow-auto rounded-xl p-1 shadow-2xl">
          {error ? (
            <p className="p-3 text-sm text-danger">{error}</p>
          ) : indexing ? (
            <p className="p-3 text-sm text-muted">Search index is still building, try again shortly.</p>
          ) : results.length === 0 ? (
            <p className="p-3 text-sm text-muted">No matches</p>
          ) : (
            results.map((entry) => (
              <button
                key={entry.path}
                onClick={() => select(entry)}
                className="flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-white/5"
              >
                <FileTypeIcon entry={entry} className="h-4 w-4 flex-shrink-0 text-accent" />
                <span className="flex-1 truncate">{entry.name}</span>
                <span className="truncate text-xs text-muted">{entry.path}</span>
                {!entry.isDir && <span className="flex-shrink-0 text-xs text-muted">{formatBytes(entry.size)}</span>}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  )
}
