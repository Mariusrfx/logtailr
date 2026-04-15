import { useCallback, useEffect, useState } from "react"
import { Check, X } from "lucide-react"
import { api } from "@/lib/api"
import { NoDatabaseBanner, isNoDatabaseError } from "./NoDatabaseBanner"

const SETTING_KEYS = [
  { key: "global.level", label: "Log Level", description: "Minimum log level to display" },
  { key: "global.regex", label: "Regex Filter", description: "Global regex pattern filter" },
  { key: "global.output", label: "Output Mode", description: "Default output (console, json, file)" },
  { key: "global.output_path", label: "Output Path", description: "File path for file output" },
  { key: "global.show_health", label: "Show Health", description: "Display health status in console" },
  { key: "global.aggregate", label: "Aggregate", description: "Enable log aggregation" },
  { key: "global.aggregate_window", label: "Aggregate Window", description: "Time window for aggregation (e.g. 5s)" },
  { key: "alerts.default_cooldown", label: "Alert Cooldown", description: "Default alert cooldown period (e.g. 5m)" },
  { key: "alerts.notify.console", label: "Console Alerts", description: "Print alerts to stderr" },
  { key: "alerts.notify.webhook.url", label: "Alert Webhook URL", description: "Webhook URL for alert notifications" },
]

interface SettingsTabProps {
  refreshKey: number
}

export function SettingsTab({ refreshKey }: SettingsTabProps) {
  const [settings, setSettings] = useState<Record<string, unknown>>({})
  const [loading, setLoading] = useState(true)
  const [noDb, setNoDb] = useState(false)
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [editValue, setEditValue] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchSettings = useCallback(async () => {
    setLoading(true)
    setNoDb(false)
    try {
      const data = await api.getSetting(SETTING_KEYS[0].key)
      const results: Record<string, unknown> = { [SETTING_KEYS[0].key]: data.value }
      await Promise.allSettled(
        SETTING_KEYS.slice(1).map(async ({ key }) => {
          try {
            const d = await api.getSetting(key)
            results[key] = d.value
          } catch {
            // Setting not found — leave empty
          }
        })
      )
      setSettings(results)
    } catch (err) {
      if (err instanceof Error && isNoDatabaseError(err.message)) {
        setNoDb(true)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchSettings() }, [fetchSettings, refreshKey])

  const handleSave = async (key: string) => {
    setSaving(true)
    setError(null)
    try {
      let value: unknown = editValue
      if (editValue === "true") value = true
      else if (editValue === "false") value = false
      else if (/^\d+$/.test(editValue)) value = parseInt(editValue, 10)

      await api.setSetting(key, value)
      setSettings((prev) => ({ ...prev, [key]: value }))
      setEditingKey(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed")
    } finally {
      setSaving(false)
    }
  }

  const startEdit = (key: string) => {
    const current = settings[key]
    setEditValue(current !== undefined ? String(current) : "")
    setEditingKey(key)
    setError(null)
  }

  if (loading) {
    return <div className="p-6 text-center text-text-secondary text-sm">Loading settings...</div>
  }

  if (noDb) return <NoDatabaseBanner />

  return (
    <div className="space-y-1">
      {error && (
        <div className="p-2 rounded-md bg-error/10 text-xs text-error mb-3">{error}</div>
      )}

      <div className="border border-border rounded-lg overflow-hidden divide-y divide-border">
        {SETTING_KEYS.map(({ key, label, description }) => {
          const isEditing = editingKey === key
          const currentValue = settings[key]

          return (
            <div key={key} className="flex items-center justify-between px-4 py-3 hover:bg-surface-hover transition-colors">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-text-primary">{label}</span>
                  <span className="text-[10px] font-mono text-text-secondary">{key}</span>
                </div>
                <p className="text-xs text-text-secondary mt-0.5">{description}</p>
              </div>

              <div className="shrink-0 ml-4">
                {isEditing ? (
                  <div className="flex items-center gap-1.5">
                    <input
                      type="text"
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleSave(key)
                        if (e.key === "Escape") setEditingKey(null)
                      }}
                      autoFocus
                      className="w-40 bg-background border border-border rounded-md px-2 py-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
                    />
                    <button
                      onClick={() => handleSave(key)}
                      disabled={saving}
                      className="p-1 rounded-md text-success hover:bg-success/10 transition-colors"
                    >
                      <Check className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => setEditingKey(null)}
                      className="p-1 rounded-md text-text-secondary hover:bg-surface-hover transition-colors"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => startEdit(key)}
                    className="text-sm text-text-secondary hover:text-text-primary transition-colors font-mono px-2 py-1 rounded-md hover:bg-surface-hover"
                  >
                    {currentValue !== undefined ? String(currentValue) : <span className="italic text-text-secondary/50">not set</span>}
                  </button>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
