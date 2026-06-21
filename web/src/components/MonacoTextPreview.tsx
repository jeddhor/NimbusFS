// Split into its own module so monaco-editor (large) only enters the bundle
// when a text file is actually previewed, via React.lazy in PreviewModal.
import "@/lib/monacoSetup"
import Editor from "@monaco-editor/react"

export default function MonacoTextPreview({ language, value }: { language: string; value: string }) {
  return (
    <Editor
      height="100%"
      theme="vs-dark"
      language={language}
      value={value}
      options={{ readOnly: true, minimap: { enabled: false }, fontSize: 13 }}
    />
  )
}
