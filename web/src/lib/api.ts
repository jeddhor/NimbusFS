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
}
