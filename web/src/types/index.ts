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

// --- Config Management types (F6.15) ---

export interface SourceRow {
  ID: string
  Name: string
  Type: string
  Path: string
  Container: string
  Unit: string
  Priority: string
  OutputFormat: string
  Namespace: string
  Pod: string
  LabelSelector: string
  Kubeconfig: string
  Follow: boolean
  Parser: string
  CreatedAt: string
  UpdatedAt: string
}

export interface SourceRequest {
  name: string
  type: string
  path?: string
  container?: string
  unit?: string
  priority?: string
  output_format?: string
  namespace?: string
  pod?: string
  label_selector?: string
  kubeconfig?: string
  follow?: boolean
  parser?: string
}

export interface OutputRow {
  ID: string
  Name: string
  Type: string
  Config: Record<string, unknown> | null
  Enabled: boolean
  CreatedAt: string
  UpdatedAt: string
}

export interface OutputRequest {
  name: string
  type: string
  config?: Record<string, unknown>
  enabled?: boolean
}

export interface AlertRuleRow {
  ID: string
  Name: string
  Type: string
  Severity: string
  Pattern: string
  Level: string
  Source: string
  Threshold: number
  Window: string
  Cooldown: string
  Enabled: boolean
  CreatedAt: string
  UpdatedAt: string
}

export interface AlertRuleRequest {
  name: string
  type: string
  severity: string
  pattern?: string
  level?: string
  source?: string
  threshold?: number
  window?: string
  cooldown?: string
  enabled?: boolean
}

export interface SettingValue {
  key: string
  value: unknown
}

export interface ImportResult {
  imported: {
    sources: number
    outputs: number
    alert_rules: number
  }
}
