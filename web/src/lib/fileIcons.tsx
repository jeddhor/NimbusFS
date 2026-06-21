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

export function FileTypeIcon({ entry, className }: { entry: FileEntry; className?: string }) {
  if (entry.isDir) return <Folder className={className} />
  const group = groupOf(extOf(entry.name))
  switch (group) {
    case "image":
      return <FileImage className={className} />
    case "video":
      return <FileVideo className={className} />
    case "audio":
      return <FileAudio className={className} />
    case "archive":
      return <FileArchive className={className} />
    case "code":
      return <FileCode className={className} />
    case "spreadsheet":
      return <FileSpreadsheet className={className} />
    case "pdf":
    case "document":
      return <FileText className={className} />
    default:
      return <FileGeneric className={className} />
  }
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
