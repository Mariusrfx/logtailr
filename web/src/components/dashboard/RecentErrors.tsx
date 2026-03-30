import { AlertTriangle } from "lucide-react"
import type { AlertEvent } from "@/types"
import { formatTimestamp } from "@/lib/utils"
import { cn } from "@/lib/utils"

interface RecentErrorsProps {
  events: AlertEvent[]
}

export function RecentErrors({ events }: RecentErrorsProps) {
  if (events.length === 0) {
    return (
      <div className="bg-surface rounded-lg border border-border p-6">
        <h3 className="text-sm font-medium text-text-secondary mb-4">Recent Alerts</h3>
        <div className="text-center py-4">
          <p className="text-sm text-text-secondary">No recent alerts</p>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-surface rounded-lg border border-border p-4">
      <h3 className="text-sm font-medium text-text-secondary mb-3">Recent Alerts</h3>
      <div className="space-y-2">
        {events.map((event, i) => (
          <div
            key={`${event.rule}-${event.timestamp}-${i}`}
            className="flex items-start gap-3 py-2 px-2 rounded-md hover:bg-surface-hover transition-colors"
          >
            <AlertTriangle
              className={cn(
                "h-4 w-4 mt-0.5 shrink-0",
                event.severity === "critical" ? "text-error" : "text-warning"
              )}
            />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 text-xs text-text-secondary">
                <span className="font-mono">{formatTimestamp(event.timestamp)}</span>
                {event.source && (
                  <>
                    <span className="text-border">|</span>
                    <span>{event.source}</span>
                  </>
                )}
                <span
                  className={cn(
                    "px-1.5 py-0.5 rounded text-[10px] font-medium uppercase",
                    event.severity === "critical"
                      ? "bg-error/10 text-error"
                      : "bg-warning/10 text-warning"
                  )}
                >
                  {event.severity}
                </span>
              </div>
              <p className="text-sm text-text-primary truncate mt-0.5">{event.message}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
