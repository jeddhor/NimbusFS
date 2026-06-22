import * as React from "react"
import { useParams } from "react-router-dom"
import { Home, ChevronRight, Download, FolderLock, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FileTypeIcon, previewKind, extOf } from "@/lib/fileIcons"
import { formatBytes, formatDate } from "@/lib/utils"
import { api, ApiError, type FileEntry, type ShareInfo } from "@/lib/api"

const MonacoTextPreview = React.lazy(() => import("@/components/MonacoTextPreview"))

export function ShareViewPage() {
  const { token = "" } = useParams<{ token: string }>()
  const [info, setInfo] = React.useState<ShareInfo | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(true)
  const [password, setPassword] = React.useState("")
  const [unlocking, setUnlocking] = React.useState(false)

  const loadInfo = React.useCallback(() => {
    setLoading(true)
    api
      .shareInfo(token)
      .then(setInfo)
      .catch((err) => setError(err instanceof ApiError ? err.message : "This share is unavailable"))
      .finally(() => setLoading(false))
  }, [token])

  React.useEffect(loadInfo, [loadInfo])

  async function handleUnlock(e: React.FormEvent) {
    e.preventDefault()
    setUnlocking(true)
    setError(null)
    try {
      await api.shareUnlock(token, password)
      loadInfo()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Incorrect password")
    } finally {
      setUnlocking(false)
    }
  }

  return (
    <div className="flex h-full w-full items-center justify-center bg-background p-4">
      <div className="glass w-full max-w-2xl rounded-2xl p-6 shadow-2xl">
        {loading ? (
          <p className="text-sm text-muted">Loading...</p>
        ) : error && !info ? (
          <p className="text-sm text-danger">{error}</p>
        ) : info?.requiresPassword ? (
          <form onSubmit={handleUnlock} className="flex flex-col items-center gap-3 py-6 text-center">
            <FolderLock className="text-accent" size={28} />
            <p className="text-sm text-muted">This link is password protected</p>
            <Input
              type="password"
              autoFocus
              placeholder="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="max-w-xs"
            />
            {error && <p className="text-sm text-danger">{error}</p>}
            <Button type="submit" disabled={unlocking}>
              {unlocking ? "Checking..." : "Unlock"}
            </Button>
          </form>
        ) : info ? (
          <ShareContent token={token} info={info} />
        ) : null}
      </div>
    </div>
  )
}

function ShareContent({ token, info }: { token: string; info: ShareInfo }) {
  const [subPath, setSubPath] = React.useState("")

  if (!info.isDir) {
    return <SharedFile token={token} name={info.name} path="" mode={info.mode} />
  }

  return <SharedDirectory token={token} rootName={info.name} mode={info.mode} subPath={subPath} onNavigate={setSubPath} />
}

function SharedDirectory({
  token,
  rootName,
  mode,
  subPath,
  onNavigate,
}: {
  token: string
  rootName: string
  mode: ShareInfo["mode"]
  subPath: string
  onNavigate: (path: string) => void
}) {
  const [entries, setEntries] = React.useState<FileEntry[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<string | null>(null)
  const [openEntry, setOpenEntry] = React.useState<FileEntry | null>(null)

  React.useEffect(() => {
    setLoading(true)
    api
      .shareList(token, subPath)
      .then((res) => setEntries(res.entries))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Could not load folder"))
      .finally(() => setLoading(false))
  }, [token, subPath])

  if (openEntry) {
    return (
      <div className="flex flex-col gap-3">
        <Button variant="ghost" size="sm" onClick={() => setOpenEntry(null)} className="self-start">
          ← Back
        </Button>
        <SharedFile token={token} name={openEntry.name} path={openEntry.path} mode={mode} />
      </div>
    )
  }

  const segments = subPath.split("/").filter(Boolean)

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-1 text-sm">
        <button onClick={() => onNavigate("")} className="flex items-center gap-1 text-muted hover:text-foreground">
          <Home size={14} />
          {rootName}
        </button>
        {segments.map((seg, i) => (
          <span key={i} className="flex items-center gap-1">
            <ChevronRight size={14} className="text-muted" />
            <button
              onClick={() => onNavigate(segments.slice(0, i + 1).join("/"))}
              className="text-foreground hover:underline"
            >
              {seg}
            </button>
          </span>
        ))}
      </div>

      {error && <p className="text-sm text-danger">{error}</p>}
      {loading ? (
        <p className="text-sm text-muted">Loading...</p>
      ) : entries.length === 0 ? (
        <p className="text-sm text-muted">This folder is empty</p>
      ) : (
        <div className="flex flex-col gap-1">
          {entries.map((entry) => (
            <button
              key={entry.path}
              onClick={() => (entry.isDir ? onNavigate(entry.path) : setOpenEntry(entry))}
              className="flex items-center gap-3 rounded-lg px-3 py-2 text-left text-sm hover:bg-white/5"
            >
              <FileTypeIcon entry={entry} className="h-4 w-4 text-accent" />
              <span className="flex-1 truncate">{entry.name}</span>
              {!entry.isDir && <span className="text-xs text-muted">{formatBytes(entry.size)}</span>}
              <span className="text-xs text-muted">{formatDate(entry.modified)}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function SharedFile({
  token,
  name,
  path,
  mode,
}: {
  token: string
  name: string
  path: string
  mode: ShareInfo["mode"]
}) {
  const [text, setText] = React.useState("")
  const kind = previewKind(name)
  const canDownload = mode !== "view_only"

  React.useEffect(() => {
    if (kind === "text") {
      fetch(api.shareFileUrl(token, path))
        .then((r) => r.text())
        .then(setText)
        .catch(() => setText("(could not load file)"))
    }
  }, [token, path, kind])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h1 className="truncate text-sm font-medium text-foreground">{name}</h1>
        {canDownload && (
          <a href={api.shareFileUrl(token, path, true)}>
            <Button size="sm" variant="secondary">
              <Download size={14} />
              Download
            </Button>
          </a>
        )}
      </div>

      <div className="max-h-[70vh] overflow-auto">
        {kind === "image" && (
          <img src={api.shareFileUrl(token, path)} alt={name} className="max-h-[65vh] w-full rounded-lg object-contain" />
        )}
        {kind === "video" && <video controls className="max-h-[65vh] w-full rounded-lg" src={api.shareFileUrl(token, path)} />}
        {kind === "audio" && <audio controls className="w-full" src={api.shareFileUrl(token, path)} />}
        {kind === "pdf" && (
          <iframe title={name} src={api.shareFileUrl(token, path)} className="h-[65vh] w-full rounded-lg border border-border" />
        )}
        {kind === "text" && (
          <div className="h-[60vh] overflow-hidden rounded-lg border border-border">
            <React.Suspense fallback={<div className="flex items-center gap-2 p-4 text-sm text-muted"><Loader2 size={14} className="animate-spin" />Loading editor...</div>}>
              <MonacoTextPreview language={extOf(name)} value={text} />
            </React.Suspense>
          </div>
        )}
        {!kind && <p className="text-sm text-muted">No preview available for this file. Use the download button above.</p>}
      </div>
    </div>
  )
}
