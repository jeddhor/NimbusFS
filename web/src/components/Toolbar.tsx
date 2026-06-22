import * as React from "react"
import {
  Upload,
  FolderPlus,
  FilePlus,
  Download,
  PencilLine,
  Move,
  Copy,
  Trash2,
  LayoutGrid,
  List as ListIcon,
  Share2,
  Link2,
} from "lucide-react"
import { Button } from "@/components/ui/button"

export type ViewMode = "grid" | "list"

interface ToolbarProps {
  viewMode: ViewMode
  onViewModeChange: (m: ViewMode) => void
  selectedCount: number
  onUpload: (files: FileList) => void
  onUploadFolder: (files: FileList) => void
  onNewFolder: () => void
  onNewFile: () => void
  onDownload: () => void
  onRename: () => void
  onMove: () => void
  onCopy: () => void
  onDelete: () => void
  onShare: () => void
  onShowShares: () => void
  sharingEnabled: boolean
}

export function Toolbar(props: ToolbarProps) {
  const fileInputRef = React.useRef<HTMLInputElement>(null)
  const folderInputRef = React.useRef<HTMLInputElement>(null)

  return (
    <div className="glass flex flex-wrap items-center gap-2 rounded-xl p-2">
      <Button size="sm" variant="secondary" onClick={() => fileInputRef.current?.click()}>
        <Upload size={14} />
        Upload
      </Button>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        hidden
        onChange={(e) => e.target.files && props.onUpload(e.target.files)}
      />
      <Button size="sm" variant="secondary" onClick={() => folderInputRef.current?.click()}>
        <Upload size={14} />
        Upload Folder
      </Button>
      <input
        ref={folderInputRef}
        type="file"
        multiple
        hidden
        // @ts-expect-error webkitdirectory is non-standard but widely supported
        webkitdirectory="true"
        onChange={(e) => e.target.files && props.onUploadFolder(e.target.files)}
      />

      <Button size="sm" variant="secondary" onClick={props.onNewFolder}>
        <FolderPlus size={14} />
        New Folder
      </Button>
      <Button size="sm" variant="secondary" onClick={props.onNewFile}>
        <FilePlus size={14} />
        New File
      </Button>

      <div className="mx-1 h-5 w-px bg-border" />

      {props.selectedCount > 0 && (
        <>
          {props.selectedCount === 1 && (
            <>
              <Button size="sm" variant="ghost" onClick={props.onDownload} title="Download">
                <Download size={14} />
              </Button>
              <Button size="sm" variant="ghost" onClick={props.onRename} title="Rename">
                <PencilLine size={14} />
              </Button>
              {props.sharingEnabled && (
                <Button size="sm" variant="ghost" onClick={props.onShare} title="Share">
                  <Share2 size={14} />
                </Button>
              )}
            </>
          )}
          <Button size="sm" variant="ghost" onClick={props.onMove} title="Move">
            <Move size={14} />
          </Button>
          <Button size="sm" variant="ghost" onClick={props.onCopy} title="Copy">
            <Copy size={14} />
          </Button>
          <Button size="sm" variant="destructive" onClick={props.onDelete} title="Delete">
            <Trash2 size={14} />
          </Button>
          <span className="text-xs text-muted">{props.selectedCount} selected</span>
        </>
      )}

      <div className="ml-auto flex items-center gap-1">
        {props.sharingEnabled && (
          <Button size="sm" variant="ghost" onClick={props.onShowShares}>
            <Link2 size={14} />
            Shared Links
          </Button>
        )}
        <Button
          size="icon"
          variant={props.viewMode === "grid" ? "secondary" : "ghost"}
          onClick={() => props.onViewModeChange("grid")}
          title="Grid view"
        >
          <LayoutGrid size={16} />
        </Button>
        <Button
          size="icon"
          variant={props.viewMode === "list" ? "secondary" : "ghost"}
          onClick={() => props.onViewModeChange("list")}
          title="List view"
        >
          <ListIcon size={16} />
        </Button>
      </div>
    </div>
  )
}
