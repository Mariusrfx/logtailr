import { useCallback, useEffect, useState } from "react"
import { Bell, CheckCircle, ChevronLeft, ChevronRight } from "lucide-react"
import { api } from "@/lib/api"
import type { AlertEventRow } from "@/types"
import { cn, formatRelativeTime } from "@/lib/utils"

const PAGE_SIZE = 25

const severityOptions = ["", "warning", "critical"] as const

export function AlertsPage() {
  const [events, setEvents] = useState<AlertEventRow[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)
  const [severityFilter, setSeverityFilter] = useState("")
  const [ruleFilter, setRuleFilter] = useState("")

  const fetchEvents = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const params: Record<string, string | number> = {
        limit: PAGE_SIZE,
        offset,
      }
      if (severityFilter) params.severity = severityFilter
      if (ruleFilter) params.rule = ruleFilter
      const data = await api.getAlertEvents(params)
      setEvents(data.events ?? [])
      setTotal(data.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load alerts")
    } finally {
      setLoading(false)
    }
  }, [offset, severityFilter, ruleFilter])

  useEffect(() => {
    fetchEvents()
    const interval = setInterval(fetchEvents, 10_000)
    return () => clearInterval(interval)
  }, [fetchEvents])

  // Reset offset when filters change
  useEffect(() => {
    setOffset(0)
  }, [severityFilter, ruleFilter])

  const handleAcknowledge = async (id: string) => {
    try {
      await api.acknowledgeAlertEvent(id)
      setEvents((prev) =>
        prev.map((e) =>
          e.ID === id
            ? { ...e, AcknowledgedAt: new Date().toISOString() }
            : e
        )
      )
    } catch {
      // Silently fail — next poll will refresh
    }
  }

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Bell className="h-6 w-6 text-accent" />
        <h1 className="text-2xl font-bold text-text-primary">Alerts</h1>
        {total > 0 && (
          <span className="text-sm text-text-secondary">({total} events)</span>
        )}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={severityFilter}
          onChange={(e) => setSeverityFilter(e.target.value)}
          className="bg-surface border border-border rounded-md px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
        >
          <option value="">All severities</option>
          {severityOptions.filter(Boolean).map((s) => (
            <option key={s} value={s}>
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </option>
          ))}
        </select>

        <input
          type="text"
          placeholder="Filter by rule name..."
          value={ruleFilter}
          onChange={(e) => setRuleFilter(e.target.value)}
          className="bg-surface border border-border rounded-md px-3 py-1.5 text-sm text-text-primary placeholder:text-text-secondary focus:outline-none focus:ring-2 focus:ring-accent/50 w-56"
        />

        {(severityFilter || ruleFilter) && (
          <button
            onClick={() => {
              setSeverityFilter("")
              setRuleFilter("")
            }}
            className="text-xs text-text-secondary hover:text-text-primary transition-colors"
          >
            Clear filters
          </button>
        )}
      </div>

      {/* Error state */}
      {error && (
        <div className="bg-error/10 border border-error/20 rounded-lg p-4 text-sm text-error">
          {error}
        </div>
      )}

      {/* Events list */}
      <div className="bg-surface rounded-lg border border-border overflow-hidden">
        {loading && events.length === 0 ? (
          <div className="p-8 text-center text-text-secondary text-sm">
            Loading alerts...
          </div>
        ) : events.length === 0 ? (
          <div className="p-8 text-center">
            <Bell className="h-8 w-8 text-text-secondary mx-auto mb-2 opacity-50" />
            <p className="text-sm text-text-secondary">No alert events found</p>
          </div>
        ) : (
          <div className="divide-y divide-border">
            {events.map((event) => (
              <AlertEventItem
                key={event.ID}
                event={event}
                onAcknowledge={handleAcknowledge}
              />
            ))}
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-text-secondary">
            Page {currentPage} of {totalPages}
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
              className="flex items-center gap-1 px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="h-4 w-4" />
              Prev
            </button>
            <button
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total}
              className="flex items-center gap-1 px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Next
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function AlertEventItem({
  event,
  onAcknowledge,
}: {
  event: AlertEventRow
  onAcknowledge: (id: string) => void
}) {
  const isAcked = !!event.AcknowledgedAt
  const isCritical = event.Severity === "critical"

  return (
    <div
      className={cn(
        "flex items-start gap-4 px-4 py-3 transition-colors",
        isAcked ? "opacity-60" : "hover:bg-surface-hover"
      )}
    >
      {/* Severity indicator */}
      <div
        className={cn(
          "mt-1 shrink-0 w-2 h-2 rounded-full",
          isCritical ? "bg-error" : "bg-warning"
        )}
      />

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span
            className={cn(
              "px-1.5 py-0.5 rounded text-[10px] font-medium uppercase",
              isCritical
                ? "bg-error/10 text-error"
                : "bg-warning/10 text-warning"
            )}
          >
            {event.Severity}
          </span>
          <span className="text-xs font-medium text-accent">{event.RuleName}</span>
          {event.Source && (
            <span className="text-xs text-text-secondary">{event.Source}</span>
          )}
          <span className="text-xs text-text-secondary font-mono">
            {formatRelativeTime(event.FiredAt)}
          </span>
          {event.Count > 1 && (
            <span className="text-xs text-text-secondary">x{event.Count}</span>
          )}
        </div>
        <p className="text-sm text-text-primary mt-1 break-words">
          {event.Message}
        </p>
      </div>

      {/* Acknowledge button */}
      <div className="shrink-0">
        {isAcked ? (
          <div className="flex items-center gap-1 text-xs text-success">
            <CheckCircle className="h-3.5 w-3.5" />
            <span>Acked</span>
          </div>
        ) : (
          <button
            onClick={() => onAcknowledge(event.ID)}
            className="px-2.5 py-1 text-xs font-medium rounded-md border border-border text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
          >
            Acknowledge
          </button>
        )}
      </div>
    </div>
  )
}
