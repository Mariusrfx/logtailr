import { ArrowLeft, CheckCircle, AlertTriangle, XCircle, StopCircle } from "lucide-react"
import type { SourceHealth } from "@/types"
import { cn, formatRelativeTime } from "@/lib/utils"

interface SourceDetailProps {
  source: SourceHealth
  onBack: () => void
}

const statusConfig = {
  healthy: { icon: CheckCircle, color: "text-success", bg: "bg-success/10", label: "Healthy" },
  degraded: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning/10", label: "Degraded" },
  failed: { icon: XCircle, color: "text-error", bg: "bg-error/10", label: "Failed" },
  stopped: { icon: StopCircle, color: "text-text-secondary", bg: "bg-surface-hover", label: "Stopped" },
}

export function SourceDetail({ source, onBack }: SourceDetailProps) {
  const config = statusConfig[source.status] || statusConfig.stopped
  const StatusIcon = config.icon

  return (
    <div className="space-y-6">
      {/* Back button + header */}
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className="p-2 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
        <div>
          <h2 className="text-xl font-bold text-text-primary">{source.name}</h2>
          <div className="flex items-center gap-2 mt-1">
            <div className={cn("flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium", config.bg, config.color)}>
              <StatusIcon className="h-3.5 w-3.5" />
              {config.label}
            </div>
          </div>
        </div>
      </div>

      {/* Info grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <InfoCard label="Status" value={config.label}>
          <StatusIcon className={cn("h-5 w-5", config.color)} />
        </InfoCard>
        <InfoCard label="Error Count" value={String(source.error_count)}>
          {source.error_count > 0 && (
            <span className="text-xs text-error font-medium">{source.error_count} errors</span>
          )}
        </InfoCard>
        <InfoCard label="Uptime" value={source.uptime || "N/A"} />
        <InfoCard label="Last Update" value={formatRelativeTime(source.last_update)} />
      </div>

      {/* Last error */}
      {source.last_error && (
        <div className="bg-surface rounded-lg border border-border p-4">
          <h3 className="text-sm font-medium text-text-secondary mb-2">Last Error</h3>
          <div className="p-3 rounded bg-error/5 border border-error/10">
            <p className="text-sm text-error font-mono break-words">{source.last_error}</p>
          </div>
        </div>
      )}

      {/* Hint for filtered logs */}
      <div className="bg-surface rounded-lg border border-border p-4">
        <h3 className="text-sm font-medium text-text-secondary mb-2">Logs</h3>
        <p className="text-sm text-text-secondary">
          View logs for this source in the{" "}
          <a href="/logs" className="text-accent hover:underline">Log Viewer</a>
          {" "}with the source filter set to "{source.name}".
        </p>
      </div>
    </div>
  )
}

function InfoCard({ label, value, children }: { label: string; value: string; children?: React.ReactNode }) {
  return (
    <div className="bg-surface rounded-lg border border-border p-4">
      <p className="text-xs text-text-secondary mb-1">{label}</p>
      <p className="text-lg font-semibold text-text-primary">{value}</p>
      {children}
    </div>
  )
}
