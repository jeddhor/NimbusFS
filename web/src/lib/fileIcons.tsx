import {
  Folder,
  FileText,
  FileImage,
  FileVideo,
  FileAudio,
  FileArchive,
  FileCode,
  FileSpreadsheet,
  File as FileGeneric,
} from "lucide-react"
import type { FileEntry } from "@/lib/api"
import { cn } from "@/lib/utils"

const EXT_GROUPS: Record<string, string[]> = {
  image: ["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico", "avif"],
  video: ["mp4", "webm", "ogg", "mov", "mkv", "avi"],
  audio: ["mp3", "wav", "flac", "aac", "m4a", "opus"],
  pdf: ["pdf"],
  archive: ["zip", "tar", "gz", "tgz", "rar", "7z", "bz2", "xz"],
  code: ["js", "ts", "tsx", "jsx", "go", "py", "rs", "c", "cpp", "h", "java", "rb", "sh", "json", "yaml", "yml", "toml", "html", "css", "md"],
  spreadsheet: ["xls", "xlsx", "csv", "ods"],
  document: ["doc", "docx", "odt", "txt", "rtf"],
}

export function extOf(name: string): string {
  const idx = name.lastIndexOf(".")
  if (idx < 0 || idx === name.length - 1) return ""
  return name.slice(idx + 1).toLowerCase()
}

function groupOf(ext: string): string | null {
  for (const [group, exts] of Object.entries(EXT_GROUPS)) {
    if (exts.includes(ext)) return group
  }
  return null
}

const GROUP_COLORS: Record<string, string> = {
  folder: "text-sky-400",
  image: "text-pink-400",
  video: "text-fuchsia-400",
  audio: "text-emerald-400",
  archive: "text-amber-400",
  code: "text-cyan-400",
  spreadsheet: "text-green-400",
  pdf: "text-red-400",
  document: "text-indigo-400",
  generic: "text-muted",
}

export function FileTypeIcon({ entry, className }: { entry: FileEntry; className?: string }) {
  if (entry.isDir) return <Folder className={cn(GROUP_COLORS.folder, className)} />
  const group = groupOf(extOf(entry.name)) ?? "generic"
  const color = GROUP_COLORS[group] ?? GROUP_COLORS.generic
  switch (group) {
    case "image":
      return <FileImage className={cn(color, className)} />
    case "video":
      return <FileVideo className={cn(color, className)} />
    case "audio":
      return <FileAudio className={cn(color, className)} />
    case "archive":
      return <FileArchive className={cn(color, className)} />
    case "code":
      return <FileCode className={cn(color, className)} />
    case "spreadsheet":
      return <FileSpreadsheet className={cn(color, className)} />
    case "pdf":
    case "document":
      return <FileText className={cn(color, className)} />
    default:
      return <FileGeneric className={cn(color, className)} />
  }
}

// thumbnailExts mirrors internal/thumbnail's supported extensions exactly —
// requesting a thumbnail for anything else would just 404.
const THUMBNAIL_EXTS = new Set(["jpg", "jpeg", "png", "gif", "webp", "mp4", "webm", "mov", "mkv", "pdf"])

export function thumbnailable(name: string): boolean {
  return THUMBNAIL_EXTS.has(extOf(name))
}

// fileTypeLabel produces a human-readable type name for a file, e.g.
// "Python script" instead of "PY File". `names` comes from /api/file-types,
// which is sourced from the system's shared-mime-info database (see
// internal/mimetypes) rather than a hand-maintained list here.
export function fileTypeLabel(entry: FileEntry, names: Record<string, string>): string {
  if (entry.isDir) return "Folder"
  const ext = extOf(entry.name)
  if (ext && names[ext]) return names[ext]
  return ext ? `${ext.toUpperCase()} File` : "File"
}

export function previewKind(name: string): "image" | "video" | "audio" | "pdf" | "text" | null {
  const ext = extOf(name)
  if (EXT_GROUPS.image.includes(ext)) return "image"
  if (EXT_GROUPS.video.includes(ext)) return "video"
  if (EXT_GROUPS.audio.includes(ext)) return "audio"
  if (ext === "pdf") return "pdf"
  if (EXT_GROUPS.code.includes(ext) || EXT_GROUPS.document.includes(ext)) return "text"
  return null
}
