import * as React from "react"
import { Eye, Download, FolderOpen, PencilLine, Move, Copy, Share2, Trash2, FolderPlus, FilePlus } from "lucide-react"
import { Sidebar } from "@/components/Sidebar"
import { Breadcrumbs } from "@/components/Breadcrumbs"
import { Toolbar, type ViewMode } from "@/components/Toolbar"
import { FileView } from "@/components/FileView"
import { PreviewPane } from "@/components/PreviewPane"
import { ContextMenu, type ContextMenuEntry } from "@/components/ui/context-menu"
import { InputDialog } from "@/components/InputDialog"
import { ShareDialog } from "@/components/ShareDialog"
import { SharedLinksDialog } from "@/components/SharedLinksDialog"
import { SearchBar } from "@/components/SearchBar"
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

interface ContextMenuState {
  x: number
  y: number
  entry: FileEntry | null
}

export function BrowserPage() {
  const [path, setPath] = React.useState("/")
  const [entries, setEntries] = React.useState<FileEntry[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState<string | null>(null)
  const [viewMode, setViewMode] = React.useState<ViewMode>("grid")
  const [selected, setSelected] = React.useState<Set<string>>(new Set())
  const [preview, setPreview] = React.useState<FileEntry | null>(null)
  const [dialog, setDialog] = React.useState<DialogState>({ kind: "none" })
  const [contextMenu, setContextMenu] = React.useState<ContextMenuState | null>(null)
  const [anchorIndex, setAnchorIndex] = React.useState<number | null>(null)
  const [dragActive, setDragActive] = React.useState(false)
  const [sharingEnabled, setSharingEnabled] = React.useState(false)
  const [searchEnabled, setSearchEnabled] = React.useState(false)

  React.useEffect(() => {
    api
      .features()
      .then((f) => {
        setSharingEnabled(f.sharing)
        setSearchEnabled(f.search)
      })
      .catch(() => {
        setSharingEnabled(false)
        setSearchEnabled(false)
      })
  }, [])

  const refresh = React.useCallback((p: string) => {
    setLoading(true)
    setError(null)
    api
      .list(p)
      .then((res) => {
        setEntries(res.entries)
        setSelected(new Set())
        setAnchorIndex(null)
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

  function toggleSelect(entry: FileEntry, index: number, e: React.MouseEvent) {
    if (e.shiftKey && anchorIndex !== null) {
      const [start, end] = anchorIndex < index ? [anchorIndex, index] : [index, anchorIndex]
      setSelected(new Set(entries.slice(start, end + 1).map((en) => en.path)))
      return
    }
    const additive = e.metaKey || e.ctrlKey
    setSelected((prev) => {
      const next = additive ? new Set(prev) : new Set<string>()
      if (next.has(entry.path)) next.delete(entry.path)
      else next.add(entry.path)
      return next
    })
    setAnchorIndex(index)
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

  function handleContextMenu(entry: FileEntry | null, e: React.MouseEvent) {
    if (!entry) {
      setSelected(new Set())
    } else if (!selected.has(entry.path)) {
      setSelected(new Set([entry.path]))
    }
    setContextMenu({ x: e.clientX, y: e.clientY, entry })
  }

  function buildContextMenuItems(entry: FileEntry | null): ContextMenuEntry[] {
    if (!entry) {
      return [
        { label: "New Folder", icon: <FolderPlus size={14} />, onClick: () => setDialog({ kind: "newFolder" }) },
        { label: "New File", icon: <FilePlus size={14} />, onClick: () => setDialog({ kind: "newFile" }) },
      ]
    }

    const count = selected.size || 1
    const items: ContextMenuEntry[] = []

    if (count <= 1) {
      if (entry.isDir) {
        items.push({ label: "Open", icon: <FolderOpen size={14} />, onClick: () => navigate(entry.path) })
      } else {
        if (previewKind(entry.name)) {
          items.push({ label: "Preview", icon: <Eye size={14} />, onClick: () => setPreview(entry) })
        }
        items.push({
          label: "Download",
          icon: <Download size={14} />,
          onClick: () => {
            window.location.href = api.fileUrl(entry.path, true)
          },
        })
      }
      items.push({ separator: true })
      items.push({ label: "Rename", icon: <PencilLine size={14} />, onClick: () => setDialog({ kind: "rename", entry }) })
    }

    items.push({ label: "Move", icon: <Move size={14} />, onClick: () => setDialog({ kind: "move", paths: [...selected] }) })
    items.push({ label: "Copy", icon: <Copy size={14} />, onClick: () => setDialog({ kind: "copy", paths: [...selected] }) })
    if (sharingEnabled && count <= 1) {
      items.push({ label: "Share", icon: <Share2 size={14} />, onClick: () => setDialog({ kind: "share", path: entry.path }) })
    }
    items.push({ separator: true })
    items.push({
      label: count > 1 ? `Delete ${count} items` : "Delete",
      icon: <Trash2 size={14} />,
      danger: true,
      onClick: handleDelete,
    })
    return items
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
        <div className="flex items-center gap-3">
          <div className="flex-1">
            <Breadcrumbs path={path} onNavigate={navigate} />
          </div>
          {searchEnabled && <SearchBar onNavigate={navigate} onOpen={openEntry} />}
        </div>

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
            <FileView
              entries={entries}
              viewMode={viewMode}
              selected={selected}
              onSelect={toggleSelect}
              onOpen={openEntry}
              onContextMenu={handleContextMenu}
            />
          )}
        </div>
      </div>

      <PreviewPane entry={preview} onClose={() => setPreview(null)} />

      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={buildContextMenuItems(contextMenu.entry)}
          onClose={() => setContextMenu(null)}
        />
      )}

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
