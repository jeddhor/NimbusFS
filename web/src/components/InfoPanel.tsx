import * as React from "react"
import { FileTypeIcon, extOf } from "@/lib/fileIcons"
import { formatBytes, formatDate } from "@/lib/utils"
import type { FileEntry } from "@/lib/api"

function typeLabel(entry: FileEntry): string {
  if (entry.isDir) return "Folder"
  const ext = extOf(entry.name)
  return ext ? `${ext.toUpperCase()} File` : "File"
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1">
      <span className="flex-shrink-0 text-muted">{label}</span>
      <span
        className="truncate text-right text-foreground"
        title={typeof value === "string" ? value : undefined}
      >
        {value}
      </span>
    </div>
  )
}

export function InfoPanel({ entries }: { entries: FileEntry[] }) {
  if (entries.length === 0) {
    return (
      <div className="flex-1 border-t border-border pt-4">
        <p className="px-1 text-xs text-muted">Select a file or folder to see its details.</p>
      </div>
    )
  }

  if (entries.length === 1) {
    const e = entries[0]
    return (
      <div className="flex-1 overflow-y-auto border-t border-border pt-4">
        <div className="mb-2 flex items-center gap-2 px-1">
          <FileTypeIcon entry={e} className="h-5 w-5 flex-shrink-0" />
          <span className="truncate text-sm font-medium text-foreground" title={e.name}>
            {e.name}
          </span>
        </div>
        <div className="flex flex-col px-1 text-xs">
          <Row label="Type" value={typeLabel(e)} />
          <Row label="Size" value={e.isDir ? "—" : formatBytes(e.size)} />
          <Row label="Modified" value={formatDate(e.modified)} />
          <Row label="Owner" value={e.owner} />
          <Row label="Group" value={e.group} />
          <Row label="Permissions" value={<span className="font-mono">{e.mode}</span>} />
          <Row label="Path" value={e.path} />
        </div>
      </div>
    )
  }

  const dirCount = entries.filter((e) => e.isDir).length
  const fileCount = entries.length - dirCount
  const totalSize = entries.reduce((sum, e) => sum + (e.isDir ? 0 : e.size), 0)

  return (
    <div className="flex-1 overflow-y-auto border-t border-border pt-4">
      <p className="mb-2 px-1 text-sm font-medium text-foreground">{entries.length} items selected</p>
      <div className="flex flex-col px-1 text-xs">
        {dirCount > 0 && <Row label="Folders" value={dirCount} />}
        {fileCount > 0 && <Row label="Files" value={fileCount} />}
        <Row label="Total size" value={formatBytes(totalSize)} />
      </div>
    </div>
  )
}
