import { useCallback, useEffect, useState } from "react"
import { Plus, Pencil, Trash2, CheckCircle, XCircle } from "lucide-react"
import { api } from "@/lib/api"
import type { AlertRuleRow, AlertRuleRequest } from "@/types"
import { cn } from "@/lib/utils"
import { DeleteConfirmModal } from "./DeleteConfirmModal"
import { useToast } from "@/hooks/useToast"
import { ListItemSkeleton } from "@/components/ui/Skeleton"
import { NoDatabaseBanner, isNoDatabaseError } from "./NoDatabaseBanner"

const RULE_TYPES = ["pattern", "level", "error_rate", "health_change"] as const
const SEVERITIES = ["warning", "critical"] as const

const typeFields: Record<string, string[]> = {
  pattern: ["pattern", "source"],
  level: ["level", "source"],
  error_rate: ["threshold", "window", "source"],
  health_change: ["source"],
}

const emptyForm: AlertRuleRequest = { name: "", type: "pattern", severity: "warning", enabled: true }

interface AlertRulesTabProps {
  refreshKey: number
}

export function AlertRulesTab({ refreshKey }: AlertRulesTabProps) {
  const [rules, setRules] = useState<AlertRuleRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ id?: string; form: AlertRuleRequest } | null>(null)
  const [deleting, setDeleting] = useState<AlertRuleRow | null>(null)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const { toast } = useToast()

  const fetchRules = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.listAlertRules()
      setRules(data.rules ?? [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load alert rules")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchRules() }, [fetchRules, refreshKey])

  const handleSave = async () => {
    if (!editing) return
    setSaving(true)
    setFormError(null)
    try {
      const data = { ...editing.form }
      if (data.threshold) data.threshold = Number(data.threshold)
      if (editing.id) {
        await api.updateAlertRule(editing.id, data)
      } else {
        await api.createAlertRule(data)
      }
      toast("success", editing.id ? "Alert rule updated" : "Alert rule created")
      setEditing(null)
      fetchRules()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Save failed")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deleting) return
    try {
      await api.deleteAlertRule(deleting.ID)
      toast("success", `Alert rule "${deleting.Name}" deleted`)
      setDeleting(null)
      fetchRules()
    } catch {
      toast("error", "Failed to delete alert rule")
      setDeleting(null)
    }
  }

  const startEdit = (rule: AlertRuleRow) => {
    setEditing({
      id: rule.ID,
      form: {
        name: rule.Name,
        type: rule.Type,
        severity: rule.Severity,
        pattern: rule.Pattern,
        level: rule.Level,
        source: rule.Source,
        threshold: rule.Threshold,
        window: rule.Window,
        cooldown: rule.Cooldown,
        enabled: rule.Enabled,
      },
    })
    setFormError(null)
  }

  const updateForm = (field: string, value: string | boolean | number) => {
    if (!editing) return
    setEditing({ ...editing, form: { ...editing.form, [field]: value } })
  }

  if (loading && rules.length === 0) {
    return (
      <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
        {Array.from({ length: 3 }).map((_, i) => <ListItemSkeleton key={i} />)}
      </div>
    )
  }

  if (error && rules.length === 0) {
    if (isNoDatabaseError(error)) return <NoDatabaseBanner />
    return <div className="p-6 text-center text-text-secondary text-sm">{error}</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-text-secondary">{rules.length} rule(s)</span>
        <button
          onClick={() => { setEditing({ form: { ...emptyForm } }); setFormError(null) }}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 transition-colors"
        >
          <Plus className="h-4 w-4" /> Add Rule
        </button>
      </div>

      {/* Form */}
      {editing && (
        <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
          <h4 className="text-sm font-medium text-text-primary">
            {editing.id ? "Edit Alert Rule" : "New Alert Rule"}
          </h4>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-text-secondary mb-1">Name</label>
              <input
                type="text"
                value={editing.form.name}
                onChange={(e) => updateForm("name", e.target.value)}
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              />
            </div>
            <div>
              <label className="block text-xs text-text-secondary mb-1">Type</label>
              <select
                value={editing.form.type}
                onChange={(e) => updateForm("type", e.target.value)}
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              >
                {RULE_TYPES.map((t) => <option key={t} value={t}>{t.replace(/_/g, " ")}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-text-secondary mb-1">Severity</label>
              <select
                value={editing.form.severity}
                onChange={(e) => updateForm("severity", e.target.value)}
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              >
                {SEVERITIES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
          </div>

          {/* Dynamic fields */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {(typeFields[editing.form.type] || []).map((f) => (
              <div key={f}>
                <label className="block text-xs text-text-secondary mb-1 capitalize">{f.replace(/_/g, " ")}</label>
                <input
                  type={f === "threshold" ? "number" : "text"}
                  value={(editing.form as Record<string, unknown>)[f] as string ?? ""}
                  onChange={(e) => updateForm(f, f === "threshold" ? Number(e.target.value) : e.target.value)}
                  placeholder={f === "window" ? "e.g. 5m" : f === "pattern" ? "regex pattern" : ""}
                  className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
                />
              </div>
            ))}
            <div>
              <label className="block text-xs text-text-secondary mb-1">Cooldown</label>
              <input
                type="text"
                value={editing.form.cooldown || ""}
                onChange={(e) => updateForm("cooldown", e.target.value)}
                placeholder="e.g. 5m"
                className="w-full bg-background border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
              />
            </div>
          </div>

          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={editing.form.enabled ?? true}
              onChange={(e) => updateForm("enabled", e.target.checked)}
              className="h-4 w-4 rounded border-border accent-accent"
            />
            <span className="text-sm text-text-primary">Enabled</span>
          </label>

          {formError && (
            <div className="p-2 rounded-md bg-error/10 text-xs text-error">{formError}</div>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <button onClick={() => setEditing(null)} className="px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover transition-colors">
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving || !editing.form.name || !editing.form.type || !editing.form.severity}
              className="px-3 py-1.5 text-sm rounded-md bg-accent text-white hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? "Saving..." : editing.id ? "Update" : "Create"}
            </button>
          </div>
        </div>
      )}

      {/* List */}
      {rules.length === 0 ? (
        <div className="p-6 text-center text-text-secondary text-sm">No alert rules configured</div>
      ) : (
        <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
          {rules.map((r) => (
            <div key={r.ID} className="flex items-center justify-between px-4 py-3 hover:bg-surface-hover transition-colors">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-text-primary">{r.Name}</span>
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-accent/10 text-accent uppercase">{r.Type.replace(/_/g, " ")}</span>
                  <span className={cn(
                    "px-1.5 py-0.5 rounded text-[10px] font-medium uppercase",
                    r.Severity === "critical" ? "bg-error/10 text-error" : "bg-warning/10 text-warning"
                  )}>
                    {r.Severity}
                  </span>
                  {r.Enabled ? (
                    <CheckCircle className="h-3.5 w-3.5 text-success" />
                  ) : (
                    <XCircle className="h-3.5 w-3.5 text-text-secondary" />
                  )}
                </div>
                <p className="text-xs text-text-secondary mt-0.5">
                  {r.Pattern || r.Level || (r.Threshold > 0 ? `>${r.Threshold} in ${r.Window}` : r.Type)}
                  {r.Source && ` (source: ${r.Source})`}
                </p>
              </div>
              <div className="flex items-center gap-1 shrink-0 ml-4">
                <button onClick={() => startEdit(r)} className="p-1.5 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors">
                  <Pencil className="h-4 w-4" />
                </button>
                <button onClick={() => setDeleting(r)} className="p-1.5 rounded-md text-text-secondary hover:bg-error/10 hover:text-error transition-colors">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {deleting && (
        <DeleteConfirmModal
          title="Delete Alert Rule"
          itemName={deleting.Name}
          onConfirm={handleDelete}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}
