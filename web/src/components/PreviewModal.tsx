import * as React from "react"
import { Dialog } from "@/components/ui/dialog"
import { api, type FileEntry } from "@/lib/api"
import { previewKind, extOf } from "@/lib/fileIcons"

const MonacoTextPreview = React.lazy(() => import("@/components/MonacoTextPreview"))

export function PreviewModal({ entry, onClose }: { entry: FileEntry | null; onClose: () => void }) {
  const [text, setText] = React.useState<string>("")
  const kind = entry ? previewKind(entry.name) : null

  React.useEffect(() => {
    if (entry && kind === "text") {
      fetch(api.fileUrl(entry.path))
        .then((r) => r.text())
        .then(setText)
        .catch(() => setText("(could not load file)"))
    }
  }, [entry, kind])

  if (!entry) return null

  return (
    <Dialog open={!!entry} onClose={onClose} title={entry.name} className="max-w-3xl">
      <div className="max-h-[70vh] overflow-auto">
        {kind === "image" && (
          <img src={api.fileUrl(entry.path)} alt={entry.name} className="max-h-[65vh] w-full rounded-lg object-contain" />
        )}
        {kind === "video" && (
          <video controls className="max-h-[65vh] w-full rounded-lg" src={api.fileUrl(entry.path)} />
        )}
        {kind === "audio" && <audio controls className="w-full" src={api.fileUrl(entry.path)} />}
        {kind === "pdf" && (
          <iframe title={entry.name} src={api.fileUrl(entry.path)} className="h-[65vh] w-full rounded-lg border border-border" />
        )}
        {kind === "text" && (
          <div className="h-[60vh] overflow-hidden rounded-lg border border-border">
            <React.Suspense fallback={<div className="p-4 text-sm text-muted">Loading editor...</div>}>
              <MonacoTextPreview language={extOf(entry.name)} value={text} />
            </React.Suspense>
          </div>
        )}
        {!kind && <p className="text-sm text-muted">No preview available for this file type.</p>}
      </div>
    </Dialog>
  )
}
