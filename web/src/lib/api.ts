export interface FileEntry {
  name: string
  path: string
  isDir: boolean
  size: number
  modified: string
  mode: string
  owner: string
  group: string
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export type ShareMode = "both" | "view_only" | "download_only"

export interface Share {
  token: string
  path: string
  name: string
  isDir: boolean
  mode: ShareMode
  hasPassword: boolean
  createdAt: string
  expiresAt?: string
  url: string
}

export interface ShareInfo {
  name: string
  isDir: boolean
  mode: ShareMode
  requiresPassword: boolean
}

function csrfToken(): string {
  const match = document.cookie.match(/(?:^|; )nimbusfs_csrf=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : ""
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  const method = (init?.method ?? "GET").toUpperCase()
  if (method !== "GET" && method !== "HEAD") {
    headers.set("X-CSRF-Token", csrfToken())
  }
  const res = await fetch(path, { ...init, headers, credentials: "same-origin" })
  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // ignore non-JSON error bodies
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  login(username: string, password: string, remember: boolean) {
    return request<{ username: string }>("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, remember }),
    })
  },
  logout() {
    return request<{ status: string }>("/api/logout", { method: "POST" })
  },
  me() {
    return request<{ username: string }>("/api/me")
  },
  authMethods() {
    return request<{ pam: boolean; sshKeys: boolean; proxyAuth: boolean }>("/api/auth/methods")
  },
  features() {
    return request<{ sharing: boolean; search: boolean }>("/api/features")
  },
  fileTypes() {
    return request<Record<string, string>>("/api/file-types")
  },
  sshStart(username: string) {
    return request<{ code: string; pollToken: string; expiresIn: number }>("/api/auth/ssh/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username }),
    })
  },
  sshPoll(pollToken: string) {
    return request<{ status: "pending" | "approved"; username?: string }>(
      `/api/auth/ssh/poll?pollToken=${encodeURIComponent(pollToken)}`,
    )
  },
  list(path: string) {
    return request<{ path: string; entries: FileEntry[] }>(`/api/files?path=${encodeURIComponent(path)}`)
  },
  fileUrl(path: string, download = false) {
    const q = new URLSearchParams({ path })
    if (download) q.set("download", "1")
    return `/api/file?${q.toString()}`
  },
  thumbnailUrl(path: string, size?: number) {
    const q = new URLSearchParams({ path })
    if (size) q.set("size", String(size))
    return `/api/thumbnail?${q.toString()}`
  },
  search(q: string) {
    return request<{ entries: FileEntry[]; indexing: boolean }>(`/api/search?q=${encodeURIComponent(q)}`)
  },
  async upload(destPath: string, files: FileList | File[]) {
    const form = new FormData()
    for (const f of Array.from(files)) {
      const relPath = (f as File & { webkitRelativePath?: string }).webkitRelativePath
      form.append("files", f, relPath || f.name)
    }
    return request<{ status: string }>(`/api/upload?path=${encodeURIComponent(destPath)}`, {
      method: "POST",
      body: form,
    })
  },
  deleteFile(path: string) {
    return request<{ status: string }>(`/api/file?path=${encodeURIComponent(path)}`, { method: "DELETE" })
  },
  mkdir(path: string) {
    return request<{ status: string }>("/api/mkdir", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    })
  },
  createFile(path: string) {
    return request<{ status: string }>("/api/create-file", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    })
  },
  rename(path: string, newName: string) {
    return request<{ path: string }>("/api/rename", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, newName }),
    })
  },
  move(src: string, dest: string) {
    return request<{ status: string }>("/api/move", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ src, dest }),
    })
  },
  copy(src: string, dest: string) {
    return request<{ status: string }>("/api/copy", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ src, dest }),
    })
  },

  createShare(opts: { path: string; mode: ShareMode; password?: string; expiresInHours?: number }) {
    return request<Share>("/api/shares", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        path: opts.path,
        mode: opts.mode,
        password: opts.password ?? "",
        expiresInHours: opts.expiresInHours ?? 0,
      }),
    })
  },
  listShares() {
    return request<{ shares: Share[] }>("/api/shares")
  },
  revokeShare(token: string) {
    return request<{ status: string }>(`/api/shares/${encodeURIComponent(token)}`, { method: "DELETE" })
  },

  // Public, anonymous share-viewer endpoints (no session/CSRF involved).
  shareInfo(token: string) {
    return request<ShareInfo>(`/api/s/${encodeURIComponent(token)}/info`)
  },
  shareUnlock(token: string, password: string) {
    return request<{ status: string }>(`/api/s/${encodeURIComponent(token)}/unlock`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    })
  },
  shareList(token: string, path: string) {
    return request<{ entries: FileEntry[] }>(
      `/api/s/${encodeURIComponent(token)}/files?path=${encodeURIComponent(path)}`,
    )
  },
  shareFileUrl(token: string, path: string, download = false) {
    const q = new URLSearchParams({ path })
    if (download) q.set("download", "1")
    return `/api/s/${encodeURIComponent(token)}/file?${q.toString()}`
  },
}
