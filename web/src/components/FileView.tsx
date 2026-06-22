import * as React from "react"
import { motion } from "framer-motion"
import { cn, formatBytes, formatDate } from "@/lib/utils"
import { FileTypeIcon, thumbnailable } from "@/lib/fileIcons"
import { api, type FileEntry } from "@/lib/api"
import type { ViewMode } from "@/components/Toolbar"

interface FileViewProps {
  entries: FileEntry[]
  viewMode: ViewMode
  selected: Set<string>
  onSelect: (path: string, additive: boolean) => void
  onOpen: (entry: FileEntry) => void
}

function GridThumbnail({ entry }: { entry: FileEntry }) {
  const [failed, setFailed] = React.useState(false)

  if (entry.isDir || !thumbnailable(entry.name) || failed) {
    return <FileTypeIcon entry={entry} className="h-10 w-10 text-accent" />
  }

  return (
    <img
      src={api.thumbnailUrl(entry.path)}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className="h-16 w-16 rounded-md object-cover"
    />
  )
}

export function FileView({ entries, viewMode, selected, onSelect, onOpen }: FileViewProps) {
  if (entries.length === 0) {
    return <div className="flex flex-1 items-center justify-center text-sm text-muted">This folder is empty</div>
  }

  if (viewMode === "grid") {
    return (
      <div className="grid grid-cols-2 gap-3 overflow-auto p-1 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        {entries.map((entry) => (
          <motion.button
            key={entry.path}
            layout
            onClick={(e) => onSelect(entry.path, e.metaKey || e.ctrlKey || e.shiftKey)}
            onDoubleClick={() => onOpen(entry)}
            whileHover={{ y: -2 }}
            className={cn(
              "glass flex flex-col items-center gap-2 rounded-xl p-4 text-center transition-colors",
              selected.has(entry.path) ? "ring-2 ring-accent" : "hover:bg-white/5",
            )}
          >
            <GridThumbnail entry={entry} />
            <span className="w-full truncate text-xs text-foreground">{entry.name}</span>
          </motion.button>
        ))}
      </div>
    )
  }

  return (
    <div className="glass flex-1 overflow-auto rounded-xl">
      <table className="w-full text-left text-sm">
        <thead className="sticky top-0 bg-surface/80 text-xs uppercase text-muted backdrop-blur">
          <tr>
            <th className="px-4 py-2">Name</th>
            <th className="px-4 py-2">Size</th>
            <th className="px-4 py-2">Modified</th>
            <th className="px-4 py-2">Owner</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <tr
              key={entry.path}
              onClick={(e) => onSelect(entry.path, e.metaKey || e.ctrlKey || e.shiftKey)}
              onDoubleClick={() => onOpen(entry)}
              className={cn(
                "cursor-pointer border-t border-border transition-colors",
                selected.has(entry.path) ? "bg-accent/15" : "hover:bg-white/5",
              )}
            >
              <td className="flex items-center gap-2 px-4 py-2">
                <FileTypeIcon entry={entry} className="h-4 w-4 text-accent" />
                {entry.name}
              </td>
              <td className="px-4 py-2 text-muted">{entry.isDir ? "—" : formatBytes(entry.size)}</td>
              <td className="px-4 py-2 text-muted">{formatDate(entry.modified)}</td>
              <td className="px-4 py-2 text-muted">{entry.owner}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
