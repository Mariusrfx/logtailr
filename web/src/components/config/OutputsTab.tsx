import { useCallback, useEffect, useState } from "react"
import { Plus, Pencil, Trash2, CheckCircle, XCircle } from "lucide-react"
import { api } from "@/lib/api"
import type { OutputRow, OutputRequest } from "@/types"
import { DeleteConfirmModal } from "./DeleteConfirmModal"
import { useToast } from "@/hooks/useToast"
import { ListItemSkeleton } from "@/components/ui/Skeleton"
import { NoDatabaseBanner, isNoDatabaseError } from "./NoDatabaseBanner"

const OUTPUT_TYPES = ["opensearch", "webhook", "file"] as const

const typeConfigFields: Record<string, { key: string; label: string; sensitive?: boolean }[]> = {
  opensearch: [
    { key: "hosts", label: "Hosts (comma separated)" },
    { key: "index", label: "Index pattern" },
    { key: "username", label: "Username" },
    { key: "password", label: "Password", sensitive: true },
    { key: "bulk_size", label: "Bulk size" },
    { key: "flush_interval", label: "Flush interval" },
  ],
  webhook: [
    { key: "url", label: "Webhook URL" },
    { key: "min_level", label: "Min level" },
    { key: "batch_size", label: "Batch size" },
    { key: "batch_timeout", label: "Batch timeout" },
  ],
  file: [
    { key: "path", label: "File path" },
    { key: "max_size", label: "Max size (e.g. 100MB)" },
    { key: "max_age", label: "Max age (e.g. 7d)" },
    { key: "compress", label: "Compress rotated" },
  ],
}

interface OutputsTabProps {
  refreshKey: number
}

export function OutputsTab({ refreshKey }: OutputsTabProps) {
  const [outputs, setOutputs] = useState<OutputRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ id?: string; form: OutputRequest; configFields: Record<string, string> } | null>(null)
  const [deleting, setDeleting] = useState<OutputRow | null>(null)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const { toast } = useToast()

  const fetchOutputs = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.listOutputs()
      setOutputs(data.outputs ?? [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load outputs")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchOutputs() }, [fetchOutputs, refreshKey])

  const buildConfig = (type: string, fields: Record<string, string>): Record<string, unknown> => {
    const config: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(fields)) {
      if (!v) continue
      if (k === "hosts") {
        config[k] = v.split(",").map((h) => h.trim())
      } else if (k === "bulk_size" || k === "batch_size") {
        config[k] = parseInt(v, 10) || 0
      } else if (k === "compress") {
        config[k] = v === "true"
      } else {
        config[k] = v
      }
    }
    return config
  }

  const extractConfigFields = (type: string, config: Record<string, unknown> | null): Record<string, string> => {
    const fields: Record<string, string> = {}
    const defs = typeConfigFields[type] || []
    for (const d of defs) {
      const val = config?.[d.key]
      if (Array.isArray(val)) {
        fields[d.key] = val.join(", ")
      } else if (val !== undefined && val !== null) {
        fields[d.key] = String(val)
      } else {
        fields[d.key] = ""
      }
    }
    return fields
  }

  const handleSave = async () => {
    if (!editing) return
    setSaving(true)
    setFormError(null)
    try {
      const data: OutputRequest = {
        name: editing.form.name,
        type: editing.form.type,
        enabled: editing.form.enabled,
        config: buildConfig(editing.form.type, editing.configFields),
      }
      if (editing.id) {
        await api.updateOutput(editing.id, data)
      } else {
        await api.createOutput(data)
      }
      toast("success", editing.id ? "Output updated" : "Output created")
      setEditing(null)
      fetchOutputs()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Save failed")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deleting) return
    try {
      await api.deleteOutput(deleting.ID)
      toast("success", `Output "${deleting.Name}" deleted`)
      setDeleting(null)
      fetchOutputs()
    } catch {
      toast("error", "Failed to delete output")
      setDeleting(null)
    }
  }

  const startEdit = (output: OutputRow) => {
    setEditing({
      id: output.ID,
      form: { name: output.Name, type: output.Type, enabled: output.Enabled },
      configFields: extractConfigFields(output.Type, output.Config),
    })
    setFormError(null)
  }

  const startCreate = () => {
    setEditing({
      form: { name: "", type: "opensearch", enabled: true },
      configFields: {},
    })
    setFormError(null)
  }

  if (loading && outputs.length === 0) {
    return (
      <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
        {Array.from({ length: 3 }).map((_, i) => <ListItemSkeleton key={i} />)}
      </div>
    )
  }

  if (error && outputs.length === 0) {
    if (isNoDatabaseError(error)) return <NoDatabaseBanner />
    return <div className="p-6 text-center text-text-secondary text-sm">{error}</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-text-secondary">{outputs.length} output(s)</span>
        <button
          onClick={startCreate}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 transition-colors"
        >
          <Plus className="h-4 w-4" /> Add Output
        </button>
      </div>

      {/* Form */}
      {editing && (
        <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
          <h4 className="text-sm font-medium text-text-primary">
            {editing.id ? "Edit Output" : "New Output"}
          </h4>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-text-secondary mb-1">Name</label>
              <input
                type="text"
                value={editing.form.name}
                onChange={(e) => setEditing({ ...editing, form: { ...editing.form, name: e.target.value } })}
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              />
            </div>
            <div>
              <label className="block text-xs text-text-secondary mb-1">Type</label>
              <select
                value={editing.form.type}
                onChange={(e) => setEditing({
                  ...editing,
                  form: { ...editing.form, type: e.target.value },
                  configFields: extractConfigFields(e.target.value, null),
                })}
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              >
                {OUTPUT_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </div>
            <div className="flex items-end">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={editing.form.enabled ?? true}
                  onChange={(e) => setEditing({ ...editing, form: { ...editing.form, enabled: e.target.checked } })}
                  className="h-4 w-4 rounded border-border accent-accent"
                />
                <span className="text-sm text-text-primary">Enabled</span>
              </label>
            </div>
          </div>

          {/* Dynamic config fields */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {(typeConfigFields[editing.form.type] || []).map((f) => (
              <div key={f.key}>
                <label className="block text-xs text-text-secondary mb-1">{f.label}</label>
                <input
                  type={f.sensitive ? "password" : "text"}
                  value={editing.configFields[f.key] || ""}
                  onChange={(e) => setEditing({
                    ...editing,
                    configFields: { ...editing.configFields, [f.key]: e.target.value },
                  })}
                  placeholder={f.sensitive ? "****" : ""}
                  className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
                />
              </div>
            ))}
          </div>

          {formError && (
            <div className="p-2 rounded-md bg-error/10 text-xs text-error">{formError}</div>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <button onClick={() => setEditing(null)} className="px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover transition-colors">
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving || !editing.form.name || !editing.form.type}
              className="px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? "Saving..." : editing.id ? "Update" : "Create"}
            </button>
          </div>
        </div>
      )}

      {/* List */}
      {outputs.length === 0 ? (
        <div className="p-6 text-center text-text-secondary text-sm">No outputs configured</div>
      ) : (
        <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
          {outputs.map((o) => (
            <div key={o.ID} className="flex items-center justify-between px-4 py-3 hover:bg-surface-hover transition-colors">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-text-primary">{o.Name}</span>
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-accent/10 text-accent uppercase">{o.Type}</span>
                  {o.Enabled ? (
                    <CheckCircle className="h-3.5 w-3.5 text-success" />
                  ) : (
                    <XCircle className="h-3.5 w-3.5 text-text-secondary" />
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1 shrink-0 ml-4">
                <button onClick={() => startEdit(o)} className="p-1.5 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors">
                  <Pencil className="h-4 w-4" />
                </button>
                <button onClick={() => setDeleting(o)} className="p-1.5 rounded-md text-text-secondary hover:bg-error/10 hover:text-error transition-colors">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {deleting && (
        <DeleteConfirmModal
          title="Delete Output"
          itemName={deleting.Name}
          onConfirm={handleDelete}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}
