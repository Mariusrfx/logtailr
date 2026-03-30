import { NavLink } from "react-router-dom"
import {
  LayoutDashboard,
  ScrollText,
  Server,
  Settings,
  ChevronLeft,
  ChevronRight,
  Circle,
} from "lucide-react"
import type { WsStatus } from "@/hooks/useWebSocketContext"
import { cn } from "@/lib/utils"

interface SidebarProps {
  collapsed: boolean
  onToggle: () => void
  wsStatus: WsStatus
}

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/logs", icon: ScrollText, label: "Logs" },
  { to: "/sources", icon: Server, label: "Sources" },
  { to: "/config", icon: Settings, label: "Config" },
]

export function Sidebar({ collapsed, onToggle, wsStatus }: SidebarProps) {
  return (
    <aside
      className={cn(
        "flex flex-col h-full bg-surface border-r border-border transition-all duration-200 ease-in-out",
        collapsed ? "w-16" : "w-60"
      )}
    >
      {/* Logo */}
      <div className="flex items-center h-14 px-4 border-b border-border">
        <div className="flex items-center gap-2 min-w-0">
          <ScrollText className="h-6 w-6 text-accent shrink-0" />
          {!collapsed && (
            <span className="text-lg font-semibold text-text-primary truncate">
              Logtailr
            </span>
          )}
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-3 px-2 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors duration-150",
                isActive
                  ? "bg-accent/10 text-accent"
                  : "text-text-secondary hover:bg-surface-hover hover:text-text-primary"
              )
            }
          >
            <item.icon className="h-5 w-5 shrink-0" />
            {!collapsed && <span className="truncate">{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* Bottom: WS status + collapse */}
      <div className="border-t border-border px-3 py-3 space-y-2">
        <div className="flex items-center gap-2">
          <Circle
            className={cn(
              "h-2.5 w-2.5 fill-current shrink-0",
              wsStatus === "connected"
                ? "text-success"
                : wsStatus === "connecting"
                ? "text-warning"
                : "text-error"
            )}
          />
          {!collapsed && (
            <span className="text-xs text-text-secondary">
              {wsStatus === "connected"
                ? "Connected"
                : wsStatus === "connecting"
                ? "Connecting..."
                : "Disconnected"}
            </span>
          )}
        </div>
        <button
          onClick={onToggle}
          className="flex items-center justify-center w-full py-1.5 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
        >
          {collapsed ? (
            <ChevronRight className="h-4 w-4" />
          ) : (
            <ChevronLeft className="h-4 w-4" />
          )}
        </button>
      </div>
    </aside>
  )
}
