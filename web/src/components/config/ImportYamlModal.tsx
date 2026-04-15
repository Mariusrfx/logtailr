import { useCallback, useEffect, useRef, useState } from "react"
import { Upload, X } from "lucide-react"
import { api } from "@/lib/api"
import type { ImportResult } from "@/types"

interface ImportYamlModalProps {
  onClose: () => void
  onImported: () => void
}

export function ImportYamlModal({ onClose, onImported }: ImportYamlModalProps) {
  const [yaml, setYaml] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    textareaRef.current?.focus()
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("keydown", handler)
    return () => window.removeEventListener("keydown", handler)
  }, [onClose])

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => setYaml(reader.result as string)
    reader.readAsText(file)
  }, [])

  const handleImport = async () => {
    if (!yaml.trim()) return
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const res = await api.importYaml(yaml)
      setResult(res)
      onImported()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Import failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-surface border border-border rounded-lg w-full max-w-lg mx-4 shadow-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h3 className="text-lg font-semibold text-text-primary">Import YAML Configuration</h3>
          <button onClick={onClose} className="text-text-secondary hover:text-text-primary transition-colors">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-6 space-y-4">
          <div>
            <label className="flex items-center gap-2 px-3 py-2 text-sm border border-border border-dashed rounded-md cursor-pointer hover:bg-surface-hover transition-colors w-fit">
              <Upload className="h-4 w-4 text-text-secondary" />
              <span className="text-text-secondary">Upload YAML file</span>
              <input type="file" accept=".yaml,.yml" onChange={handleFileUpload} className="hidden" />
            </label>
          </div>

          <textarea
            ref={textareaRef}
            value={yaml}
            onChange={(e) => setYaml(e.target.value)}
            placeholder="Or paste your YAML configuration here..."
            rows={12}
            className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm font-mono text-text-primary placeholder:text-text-secondary focus:outline-none focus:ring-2 focus:ring-accent/50 resize-y"
          />

          {error && (
            <div className="p-3 rounded-md bg-error/10 border border-error/20 text-sm text-error">
              {error}
            </div>
          )}

          {result && (
            <div className="p-3 rounded-md bg-success/10 border border-success/20 text-sm text-success">
              Imported: {result.imported.sources} sources, {result.imported.outputs} outputs, {result.imported.alert_rules} alert rules
            </div>
          )}
        </div>

        <div className="flex justify-end gap-3 px-6 py-4 border-t border-border">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover transition-colors"
          >
            {result ? "Close" : "Cancel"}
          </button>
          {!result && (
            <button
              onClick={handleImport}
              disabled={loading || !yaml.trim()}
              className="px-4 py-2 text-sm rounded-md bg-accent text-white hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? "Importing..." : "Import"}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
