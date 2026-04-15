export interface LogLine {
  timestamp: string
  level: string
  message: string
  source: string
  fields?: Record<string, unknown>
}

export interface SourceHealth {
  name: string
  status: "healthy" | "degraded" | "failed" | "stopped"
  error_count: number
  last_error?: string
  last_update: string
  uptime?: string
}

export interface HealthSummary {
  status: string
  timestamp: string
  uptime: string
  sources: {
    total: number
    healthy: number
    degraded: number
    failed: number
    stopped: number
  }
}

export interface AlertEvent {
  id: string
  rule: string
  severity: string
  message: string
  source?: string
  timestamp: string
  count?: number
}

export interface AlertRule {
  name: string
  type: string
  severity: string
  pattern?: string
  level?: string
  source?: string
  threshold?: number
  window?: string
  cooldown?: string
  fire_count: number
  last_fired?: string
}

export interface AlertEventRow {
  ID: string
  RuleName: string
  Severity: string
  Message: string
  Source: string
  Count: number
  FiredAt: string
  AcknowledgedAt: string | null
}

export interface AlertEventsResponse {
  events: AlertEventRow[]
  total: number
}

export interface AlertEventFilter {
  severity?: string
  rule?: string
  source?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}
