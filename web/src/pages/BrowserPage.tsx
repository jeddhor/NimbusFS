import * as React from "react"
import { Sidebar } from "@/components/Sidebar"
import { Breadcrumbs } from "@/components/Breadcrumbs"
import { Toolbar, type ViewMode } from "@/components/Toolbar"
import { FileView } from "@/components/FileView"
import { PreviewModal } from "@/components/PreviewModal"
import { InputDialog } from "@/components/InputDialog"
import { ShareDialog } from "@/components/ShareDialog"
import { SharedLinksDialog } from "@/components/SharedLinksDialog"
import { previewKind } from "@/lib/fileIcons"
import { api, ApiError, type FileEntry } from "@/lib/api"
import { cn } from "@/lib/utils"

type DialogState =
  | { kind: "none" }
  | { kind: "newFolder" }
  | { kind: "newFile" }
  | { kind: "rename"; entry: FileEntry }
  | { kind: "move"; paths: string[] }
  | { kind: "copy"; paths: string[] }
  | { kind: "share"; path: string }
  | { kind: "sharedLinks" }

export function BrowserPage() {
  const [path, setPath] = React.useState("/")
  const [entries, setEntries] = React.useState<FileEntry[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<string | null>(null)
  const [viewMode, setViewMode] = React.useState<ViewMode>("grid")
  const [selected, setSelected] = React.useState<Set<string>>(new Set())
  const [preview, setPreview] = React.useState<FileEntry | null>(null)
  const [dialog, setDialog] = React.useState<DialogState>({ kind: "none" })
  const [dragActive, setDragActive] = React.useState(false)
  const [sharingEnabled, setSharingEnabled] = React.useState(false)

  React.useEffect(() => {
    api
      .features()
      .then((f) => setSharingEnabled(f.sharing))
      .catch(() => setSharingEnabled(false))
  }, [])

  const refresh = React.useCallback((p: string) => {
    setLoading(true)
    setError(null)
    api
      .list(p)
      .then((res) => {
        setEntries(res.entries)
        setSelected(new Set())
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load directory"))
      .finally(() => setLoading(false))
  }, [])

  React.useEffect(() => {
    refresh(path)
  }, [path, refresh])

  function navigate(p: string) {
    setPath(p)
  }

  function openEntry(entry: FileEntry) {
    if (entry.isDir) {
      navigate(entry.path)
      return
    }
    if (previewKind(entry.name)) {
      setPreview(entry)
    } else {
      window.location.href = api.fileUrl(entry.path, true)
    }
  }

  function toggleSelect(p: string, additive: boolean) {
    setSelected((prev) => {
      const next = additive ? new Set(prev) : new Set<string>()
      if (next.has(p)) next.delete(p)
      else next.add(p)
      return next
    })
  }

  async function handleUpload(files: FileList) {
    try {
      await api.upload(path, files)
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Upload failed")
    }
  }

  async function handleDelete() {
    if (selected.size === 0) return
    if (!window.confirm(`Delete ${selected.size} item(s)? This cannot be undone.`)) return
    try {
      for (const p of selected) await api.deleteFile(p)
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed")
    }
  }

  function handleDownload() {
    const p = [...selected][0]
    if (p) window.location.href = api.fileUrl(p, true)
  }

  async function handleRenameConfirm(newName: string) {
    if (dialog.kind !== "rename") return
    try {
      await api.rename(dialog.entry.path, newName)
      setDialog({ kind: "none" })
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Rename failed")
    }
  }

  async function handleMoveOrCopyConfirm(dest: string) {
    if (dialog.kind !== "move" && dialog.kind !== "copy") return
    try {
      for (const src of dialog.paths) {
        const destPath = dest.endsWith("/") ? dest + src.split("/").pop() : dest
        if (dialog.kind === "move") await api.move(src, destPath)
        else await api.copy(src, destPath)
      }
      setDialog({ kind: "none" })
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Operation failed")
    }
  }

  async function handleNewFolderConfirm(name: string) {
    try {
      await api.mkdir(joinPath(path, name))
      setDialog({ kind: "none" })
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create folder")
    }
  }

  async function handleNewFileConfirm(name: string) {
    try {
      await api.createFile(joinPath(path, name))
      setDialog({ kind: "none" })
      refresh(path)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create file")
    }
  }

  return (
    <div
      className="flex h-full w-full gap-4 p-4"
      onDragOver={(e) => {
        e.preventDefault()
        setDragActive(true)
      }}
      onDragLeave={() => setDragActive(false)}
      onDrop={(e) => {
        e.preventDefault()
        setDragActive(false)
        if (e.dataTransfer.files.length) handleUpload(e.dataTransfer.files)
      }}
    >
      <Sidebar onNavigateRoot={() => navigate("/")} />

      <div className="flex flex-1 flex-col gap-3 overflow-hidden">
        <Breadcrumbs path={path} onNavigate={navigate} />

        <Toolbar
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          selectedCount={selected.size}
          onUpload={handleUpload}
          onUploadFolder={handleUpload}
          onNewFolder={() => setDialog({ kind: "newFolder" })}
          onNewFile={() => setDialog({ kind: "newFile" })}
          onDownload={handleDownload}
          onRename={() => {
            const entry = entries.find((e) => e.path === [...selected][0])
            if (entry) setDialog({ kind: "rename", entry })
          }}
          onMove={() => setDialog({ kind: "move", paths: [...selected] })}
          onCopy={() => setDialog({ kind: "copy", paths: [...selected] })}
          onDelete={handleDelete}
          onShare={() => {
            const p = [...selected][0]
            if (p) setDialog({ kind: "share", path: p })
          }}
          onShowShares={() => setDialog({ kind: "sharedLinks" })}
          sharingEnabled={sharingEnabled}
        />

        {error && <div className="rounded-lg bg-danger/10 px-3 py-2 text-sm text-danger">{error}</div>}

        <div
          className={cn(
            "relative flex flex-1 flex-col overflow-auto rounded-xl",
            dragActive && "ring-2 ring-accent ring-offset-2 ring-offset-background",
          )}
        >
          {loading ? (
            <div className="flex flex-1 items-center justify-center text-sm text-muted">Loading...</div>
          ) : (
            <FileView entries={entries} viewMode={viewMode} selected={selected} onSelect={toggleSelect} onOpen={openEntry} />
          )}
        </div>
      </div>

      <PreviewModal entry={preview} onClose={() => setPreview(null)} />

      <InputDialog
        open={dialog.kind === "newFolder"}
        title="New Folder"
        label="Folder name"
        confirmLabel="Create"
        onClose={() => setDialog({ kind: "none" })}
        onConfirm={handleNewFolderConfirm}
      />
      <InputDialog
        open={dialog.kind === "newFile"}
        title="New File"
        label="File name"
        confirmLabel="Create"
        onClose={() => setDialog({ kind: "none" })}
        onConfirm={handleNewFileConfirm}
      />
      <InputDialog
        open={dialog.kind === "rename"}
        title="Rename"
        label="New name"
        defaultValue={dialog.kind === "rename" ? dialog.entry.name : ""}
        confirmLabel="Rename"
        onClose={() => setDialog({ kind: "none" })}
        onConfirm={handleRenameConfirm}
      />
      <InputDialog
        open={dialog.kind === "move"}
        title="Move to"
        label="Destination path"
        confirmLabel="Move"
        onClose={() => setDialog({ kind: "none" })}
        onConfirm={handleMoveOrCopyConfirm}
      />
      <InputDialog
        open={dialog.kind === "copy"}
        title="Copy to"
        label="Destination path"
        confirmLabel="Copy"
        onClose={() => setDialog({ kind: "none" })}
        onConfirm={handleMoveOrCopyConfirm}
      />
      <ShareDialog
        open={dialog.kind === "share"}
        path={dialog.kind === "share" ? dialog.path : null}
        onClose={() => setDialog({ kind: "none" })}
      />
      <SharedLinksDialog open={dialog.kind === "sharedLinks"} onClose={() => setDialog({ kind: "none" })} />
    </div>
  )
}

function joinPath(dir: string, name: string): string {
  return dir.endsWith("/") ? dir + name : dir + "/" + name
}
