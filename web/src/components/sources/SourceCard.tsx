import {
  CheckCircle, AlertTriangle, XCircle, StopCircle,
  FileText, Container, Cog, Terminal, Ship,
} from "lucide-react"
import type { SourceHealth } from "@/types"
import { cn, formatRelativeTime } from "@/lib/utils"

interface SourceCardProps {
  source: SourceHealth
  onClick: () => void
}

const statusConfig = {
  healthy: { icon: CheckCircle, color: "text-success", bg: "bg-success/10", border: "border-success/30", label: "Healthy", pulse: true },
  degraded: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning/10", border: "border-warning/30", label: "Degraded", pulse: false },
  failed: { icon: XCircle, color: "text-error", bg: "bg-error/10", border: "border-error/30", label: "Failed", pulse: false },
  stopped: { icon: StopCircle, color: "text-text-secondary", bg: "bg-surface-hover", border: "border-border", label: "Stopped", pulse: false },
}

const typeIcons: Record<string, typeof FileText> = {
  file: FileText,
  docker: Container,
  journalctl: Cog,
  kubernetes: Ship,
  stdin: Terminal,
}

export function SourceCard({ source, onClick }: SourceCardProps) {
  const config = statusConfig[source.status] || statusConfig.stopped
  const StatusIcon = config.icon
  const TypeIcon = typeIcons[guessType(source.name)] || FileText

  return (
    <div
      onClick={onClick}
      className="bg-surface rounded-lg border border-border p-5 hover:bg-surface-hover hover:border-text-secondary/20 transition-all duration-150 cursor-pointer"
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="p-2 rounded-lg bg-accent/10">
            <TypeIcon className="h-5 w-5 text-accent" />
          </div>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-text-primary truncate">{source.name}</h3>
            <p className="text-xs text-text-secondary">{guessType(source.name)}</p>
          </div>
        </div>
        <div
          className={cn(
            "flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border",
            config.bg, config.color, config.border
          )}
        >
          <StatusIcon className={cn("h-3.5 w-3.5", config.pulse && "animate-pulse")} />
          <span>{config.label}</span>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-3">
        <div>
          <p className="text-xs text-text-secondary">Errors</p>
          <p className={cn("text-lg font-bold tabular-nums", source.error_count > 0 ? "text-error" : "text-text-primary")}>
            {source.error_count}
          </p>
        </div>
        <div>
          <p className="text-xs text-text-secondary">Uptime</p>
          <p className="text-sm font-medium text-text-primary">{source.uptime || "-"}</p>
        </div>
      </div>

      {/* Last error */}
      {source.last_error && (
        <div className="mt-3 p-2 rounded bg-error/5 border border-error/10">
          <p className="text-xs text-error truncate" title={source.last_error}>
            {source.last_error}
          </p>
        </div>
      )}

      {/* Last update */}
      <p className="text-[11px] text-text-secondary mt-3">
        Updated {formatRelativeTime(source.last_update)}
      </p>
    </div>
  )
}

function guessType(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes("docker") || lower.includes("container")) return "docker"
  if (lower.includes("journal") || lower.includes("systemd")) return "journalctl"
  if (lower.includes("k8s") || lower.includes("kube")) return "kubernetes"
  if (lower.includes("stdin") || lower.includes("pipe")) return "stdin"
  return "file"
}
