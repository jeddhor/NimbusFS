import * as React from "react"
import { createPortal } from "react-dom"
import { motion, AnimatePresence } from "framer-motion"
import { X, ExternalLink, Download } from "lucide-react"
import { Button } from "@/components/ui/button"
import { api, type FileEntry } from "@/lib/api"
import { previewKind, extOf } from "@/lib/fileIcons"

const MonacoTextPreview = React.lazy(() => import("@/components/MonacoTextPreview"))

// A slide-over panel, not a centered dialog, so it pops out alongside the
// file list rather than blocking it — and never hits the download endpoint,
// just the same inline file URL the <img>/<video>/<iframe> tags use.
export function PreviewPane({ entry, onClose }: { entry: FileEntry | null; onClose: () => void }) {
  const [text, setText] = React.useState("")
  const kind = entry ? previewKind(entry.name) : null

  React.useEffect(() => {
    if (entry && kind === "text") {
      fetch(api.fileUrl(entry.path))
        .then((r) => r.text())
        .then(setText)
        .catch(() => setText("(could not load file)"))
    }
  }, [entry, kind])

  React.useEffect(() => {
    if (!entry) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("keydown", handleKey)
    return () => window.removeEventListener("keydown", handleKey)
  }, [entry, onClose])

  if (typeof document === "undefined") return null

  return createPortal(
    <AnimatePresence>
      {entry && (
        <React.Fragment key={entry.path}>
          <motion.div
            className="fixed inset-0 z-40 bg-black/40"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
          />
          <motion.div
            className="glass fixed right-0 top-0 z-50 flex h-full w-full max-w-xl flex-col border-l border-border p-4 shadow-2xl sm:p-5"
            initial={{ x: "100%" }}
            animate={{ x: 0 }}
            exit={{ x: "100%" }}
            transition={{ type: "spring", damping: 28, stiffness: 280 }}
          >
            <div className="mb-4 flex items-center justify-between gap-2">
              <h2 className="truncate text-sm font-semibold text-foreground">{entry.name}</h2>
              <div className="flex items-center gap-1">
                <Button
                  size="icon"
                  variant="ghost"
                  title="Pop out in a new tab"
                  onClick={() => window.open(api.fileUrl(entry.path), "_blank", "noopener,noreferrer")}
                >
                  <ExternalLink size={16} />
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  title="Download"
                  onClick={() => {
                    window.location.href = api.fileUrl(entry.path, true)
                  }}
                >
                  <Download size={16} />
                </Button>
                <Button size="icon" variant="ghost" title="Close" onClick={onClose}>
                  <X size={16} />
                </Button>
              </div>
            </div>

            <div className="flex-1 overflow-auto">
              {kind === "image" && (
                <img
                  src={api.fileUrl(entry.path)}
                  alt={entry.name}
                  className="max-h-full w-full rounded-lg object-contain"
                />
              )}
              {kind === "video" && (
                <video controls autoPlay className="max-h-full w-full rounded-lg" src={api.fileUrl(entry.path)} />
              )}
              {kind === "audio" && <audio controls autoPlay className="w-full" src={api.fileUrl(entry.path)} />}
              {kind === "pdf" && (
                <iframe
                  title={entry.name}
                  src={api.fileUrl(entry.path)}
                  className="h-full w-full rounded-lg border border-border"
                />
              )}
              {kind === "text" && (
                <div className="h-full overflow-hidden rounded-lg border border-border">
                  <React.Suspense fallback={<div className="p-4 text-sm text-muted">Loading editor...</div>}>
                    <MonacoTextPreview language={extOf(entry.name)} value={text} />
                  </React.Suspense>
                </div>
              )}
              {!kind && <p className="text-sm text-muted">No preview available for this file type.</p>}
            </div>
          </motion.div>
        </React.Fragment>
      )}
    </AnimatePresence>,
    document.body,
  )
}
