import { useLocation } from "react-router-dom"
import { Sun, Moon, Menu } from "lucide-react"
import type { HealthSummary } from "@/types"
import { cn } from "@/lib/utils"

interface HeaderProps {
  theme: "light" | "dark"
  onToggleTheme: () => void
  onToggleSidebar: () => void
  health: HealthSummary | null
}

const routeNames: Record<string, string> = {
  "/": "Dashboard",
  "/logs": "Logs",
  "/sources": "Sources",
  "/config": "Config",
}

export function Header({ theme, onToggleTheme, onToggleSidebar, health }: HeaderProps) {
  const location = useLocation()
  const title = routeNames[location.pathname] || "Logtailr"

  const sourceSummary = health
    ? `${health.sources.healthy}/${health.sources.total} sources healthy`
    : null

  return (
    <header className="sticky top-0 z-10 flex items-center h-14 px-4 bg-surface border-b border-border gap-4">
      {/* Mobile menu button */}
      <button
        onClick={onToggleSidebar}
        className="md:hidden p-1.5 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
      >
        <Menu className="h-5 w-5" />
      </button>

      {/* Breadcrumb / Title */}
      <h1 className="text-lg font-semibold text-text-primary">{title}</h1>

      <div className="flex-1" />

      {/* Health status */}
      {sourceSummary && (
        <div className="hidden sm:flex items-center gap-2">
          <div
            className={cn(
              "h-2 w-2 rounded-full",
              health?.status === "healthy"
                ? "bg-success"
                : health?.status === "degraded"
                ? "bg-warning"
                : "bg-error"
            )}
          />
          <span className="text-sm text-text-secondary">{sourceSummary}</span>
        </div>
      )}

      {/* Theme toggle */}
      <button
        onClick={onToggleTheme}
        className="p-2 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
        title={theme === "light" ? "Switch to dark mode" : "Switch to light mode"}
      >
        {theme === "light" ? (
          <Moon className="h-5 w-5" />
        ) : (
          <Sun className="h-5 w-5" />
        )}
      </button>
    </header>
  )
}
