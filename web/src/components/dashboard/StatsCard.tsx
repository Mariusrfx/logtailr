import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

interface StatsCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: ReactNode
  variant?: "default" | "success" | "warning" | "error"
}

const variantStyles = {
  default: "text-accent",
  success: "text-success",
  warning: "text-warning",
  error: "text-error",
}

export function StatsCard({ title, value, subtitle, icon, variant = "default" }: StatsCardProps) {
  return (
    <div className="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-colors duration-150">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-medium text-text-secondary">{title}</span>
        <div className={cn("h-5 w-5", variantStyles[variant])}>{icon}</div>
      </div>
      <div className="text-2xl font-bold text-text-primary tabular-nums">{value}</div>
      {subtitle && (
        <p className="text-xs text-text-secondary mt-1">{subtitle}</p>
      )}
    </div>
  )
}
