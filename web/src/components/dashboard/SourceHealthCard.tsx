import { CheckCircle, AlertTriangle, XCircle, StopCircle, FileText, Container, Cog, Terminal, Ship } from "lucide-react"
import type { SourceHealth } from "@/types"
import { cn } from "@/lib/utils"

interface SourceHealthCardProps {
  source: SourceHealth
}

const statusConfig = {
  healthy: { icon: CheckCircle, color: "text-success", bg: "bg-success/10", label: "Healthy" },
  degraded: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning/10", label: "Degraded" },
  failed: { icon: XCircle, color: "text-error", bg: "bg-error/10", label: "Failed" },
  stopped: { icon: StopCircle, color: "text-text-secondary", bg: "bg-surface-hover", label: "Stopped" },
}

function sourceTypeIcon(name: string) {
  const lower = name.toLowerCase()
  if (lower.includes("docker") || lower.includes("container")) return Container
  if (lower.includes("journal") || lower.includes("systemd")) return Cog
  if (lower.includes("k8s") || lower.includes("kube")) return Ship
  if (lower.includes("stdin") || lower.includes("pipe")) return Terminal
  return FileText
}

export function SourceHealthCard({ source }: SourceHealthCardProps) {
  const config = statusConfig[source.status] || statusConfig.stopped
  const StatusIcon = config.icon
  const TypeIcon = sourceTypeIcon(source.name)

  return (
    <div className="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-colors duration-150">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <TypeIcon className="h-4 w-4 text-text-secondary shrink-0" />
          <span className="text-sm font-medium text-text-primary truncate">{source.name}</span>
        </div>
        <div className={cn("flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium", config.bg, config.color)}>
          <StatusIcon className="h-3 w-3" />
          <span>{config.label}</span>
        </div>
      </div>

      <div className="space-y-1 text-xs text-text-secondary">
        {source.error_count > 0 && (
          <div className="flex justify-between">
            <span>Errors</span>
            <span className="text-error font-medium">{source.error_count}</span>
          </div>
        )}
        {source.uptime && (
          <div className="flex justify-between">
            <span>Uptime</span>
            <span>{source.uptime}</span>
          </div>
        )}
        {source.last_error && (
          <p className="text-error truncate mt-1" title={source.last_error}>
            {source.last_error}
          </p>
        )}
      </div>
    </div>
  )
}
