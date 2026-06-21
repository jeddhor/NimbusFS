// Configures Monaco to load from the locally bundled package and worker
// chunks instead of @monaco-editor/react's default CDN loader, so text
// preview keeps working with no outbound network access (self-contained deploy goal).
import * as monaco from "monaco-editor"
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker"
import { loader } from "@monaco-editor/react"

self.MonacoEnvironment = {
  getWorker() {
    return new EditorWorker()
  },
}

loader.config({ monaco })
