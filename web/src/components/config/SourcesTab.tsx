import { useCallback, useEffect, useState } from "react"
import { Plus, Pencil, Trash2 } from "lucide-react"
import { api } from "@/lib/api"
import type { SourceRow, SourceRequest } from "@/types"
import { cn } from "@/lib/utils"
import { DeleteConfirmModal } from "./DeleteConfirmModal"
import { NoDatabaseBanner, isNoDatabaseError } from "./NoDatabaseBanner"

const SOURCE_TYPES = ["file", "docker", "journalctl", "stdin", "kubernetes"] as const
const PARSERS = ["", "json", "logfmt", "text"] as const

const typeFields: Record<string, string[]> = {
  file: ["path"],
  docker: ["container"],
  journalctl: ["unit", "priority", "output_format"],
  stdin: [],
  kubernetes: ["namespace", "pod", "label_selector", "kubeconfig"],
}

const emptyForm: SourceRequest = { name: "", type: "file", follow: true, parser: "" }

interface SourcesTabProps {
  refreshKey: number
}

export function SourcesTab({ refreshKey }: SourcesTabProps) {
  const [sources, setSources] = useState<SourceRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ id?: string; form: SourceRequest } | null>(null)
  const [deleting, setDeleting] = useState<SourceRow | null>(null)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const fetchSources = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.listSources()
      setSources(data.sources ?? [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load sources")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchSources() }, [fetchSources, refreshKey])

  const handleSave = async () => {
    if (!editing) return
    setSaving(true)
    setFormError(null)
    try {
      if (editing.id) {
        await api.updateSource(editing.id, editing.form)
      } else {
        await api.createSource(editing.form)
      }
      setEditing(null)
      fetchSources()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Save failed")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deleting) return
    try {
      await api.deleteSource(deleting.ID)
      setDeleting(null)
      fetchSources()
    } catch {
      // next refresh will show current state
      setDeleting(null)
    }
  }

  const startEdit = (source: SourceRow) => {
    setEditing({
      id: source.ID,
      form: {
        name: source.Name,
        type: source.Type,
        path: source.Path,
        container: source.Container,
        unit: source.Unit,
        priority: source.Priority,
        output_format: source.OutputFormat,
        namespace: source.Namespace,
        pod: source.Pod,
        label_selector: source.LabelSelector,
        kubeconfig: source.Kubeconfig,
        follow: source.Follow,
        parser: source.Parser,
      },
    })
    setFormError(null)
  }

  const updateForm = (field: string, value: string | boolean) => {
    if (!editing) return
    setEditing({ ...editing, form: { ...editing.form, [field]: value } })
  }

  if (loading && sources.length === 0) {
    return <div className="p-6 text-center text-text-secondary text-sm">Loading sources...</div>
  }

  if (error && sources.length === 0) {
    if (isNoDatabaseError(error)) return <NoDatabaseBanner />
    return <div className="p-6 text-center text-text-secondary text-sm">{error}</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-text-secondary">{sources.length} source(s)</span>
        <button
          onClick={() => { setEditing({ form: { ...emptyForm } }); setFormError(null) }}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 transition-colors"
        >
          <Plus className="h-4 w-4" /> Add Source
        </button>
      </div>

      {/* Form */}
      {editing && (
        <SourceForm
          form={editing.form}
          isEdit={!!editing.id}
          saving={saving}
          error={formError}
          onChange={updateForm}
          onSave={handleSave}
          onCancel={() => setEditing(null)}
        />
      )}

      {/* List */}
      {sources.length === 0 ? (
        <div className="p-6 text-center text-text-secondary text-sm">No sources configured</div>
      ) : (
        <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
          {sources.map((s) => (
            <div key={s.ID} className="flex items-center justify-between px-4 py-3 hover:bg-surface-hover transition-colors">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-text-primary">{s.Name}</span>
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-accent/10 text-accent uppercase">{s.Type}</span>
                  {s.Parser && (
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-surface-hover text-text-secondary">{s.Parser}</span>
                  )}
                </div>
                <p className="text-xs text-text-secondary mt-0.5 truncate">
                  {s.Path || s.Container || s.Unit || s.Pod || "stdin"}
                </p>
              </div>
              <div className="flex items-center gap-1 shrink-0 ml-4">
                <button onClick={() => startEdit(s)} className="p-1.5 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors">
                  <Pencil className="h-4 w-4" />
                </button>
                <button onClick={() => setDeleting(s)} className="p-1.5 rounded-md text-text-secondary hover:bg-error/10 hover:text-error transition-colors">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {deleting && (
        <DeleteConfirmModal
          title="Delete Source"
          itemName={deleting.Name}
          onConfirm={handleDelete}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}

function SourceForm({
  form, isEdit, saving, error, onChange, onSave, onCancel,
}: {
  form: SourceRequest
  isEdit: boolean
  saving: boolean
  error: string | null
  onChange: (field: string, value: string | boolean) => void
  onSave: () => void
  onCancel: () => void
}) {
  const fields = typeFields[form.type] || []

  return (
    <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
      <h4 className="text-sm font-medium text-text-primary">{isEdit ? "Edit Source" : "New Source"}</h4>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <FormField label="Name" value={form.name} onChange={(v) => onChange("name", v)} />
        <div>
          <label className="block text-xs text-text-secondary mb-1">Type</label>
          <select
            value={form.type}
            onChange={(e) => onChange("type", e.target.value)}
            className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            {SOURCE_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>
      </div>

      {/* Dynamic fields based on type */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {fields.map((f) => (
          <FormField
            key={f}
            label={f.replace(/_/g, " ")}
            value={(form as Record<string, unknown>)[f] as string || ""}
            onChange={(v) => onChange(f, v)}
          />
        ))}
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-text-secondary mb-1">Parser</label>
          <select
            value={form.parser || ""}
            onChange={(e) => onChange("parser", e.target.value)}
            className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            {PARSERS.map((p) => <option key={p} value={p}>{p || "auto"}</option>)}
          </select>
        </div>
        <div className="flex items-end">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={form.follow ?? true}
              onChange={(e) => onChange("follow", e.target.checked)}
              className={cn("h-4 w-4 rounded border-border", "accent-accent")}
            />
            <span className="text-sm text-text-primary">Follow (tail -f)</span>
          </label>
        </div>
      </div>

      {error && (
        <div className="p-2 rounded-md bg-error/10 text-xs text-error">{error}</div>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <button onClick={onCancel} className="px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover transition-colors">
          Cancel
        </button>
        <button
          onClick={onSave}
          disabled={saving || !form.name || !form.type}
          className="px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? "Saving..." : isEdit ? "Update" : "Create"}
        </button>
      </div>
    </div>
  )
}

function FormField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <div>
      <label className="block text-xs text-text-secondary mb-1 capitalize">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
      />
    </div>
  )
}
