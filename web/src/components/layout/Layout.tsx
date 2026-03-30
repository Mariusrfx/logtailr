import { useState, useCallback } from "react"
import { Outlet } from "react-router-dom"
import { Sidebar } from "./Sidebar"
import { Header } from "./Header"
import { useTheme } from "@/hooks/useTheme"
import { useHealth } from "@/hooks/useHealth"
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts"
import { useDynamicTitle } from "@/hooks/useDynamicTitle"
import { WsProvider, useWsStatus } from "@/hooks/useWebSocketContext"
import { cn } from "@/lib/utils"

function LayoutInner() {
  const { theme, toggle: toggleTheme } = useTheme()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const { health } = useHealth()
  const wsStatus = useWsStatus()

  useKeyboardShortcuts()
  useDynamicTitle(health)

  const toggleSidebar = useCallback(() => {
    if (window.innerWidth < 768) {
      setMobileOpen((prev) => !prev)
    } else {
      setSidebarCollapsed((prev) => !prev)
    }
  }, [])

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {mobileOpen && (
        <div
          className="fixed inset-0 z-20 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      <div
        className={cn(
          "shrink-0 z-30",
          "fixed md:relative",
          "transition-transform duration-200 ease-in-out",
          mobileOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"
        )}
      >
        <Sidebar
          collapsed={sidebarCollapsed}
          onToggle={toggleSidebar}
          wsStatus={wsStatus}
        />
      </div>

      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <Header
          theme={theme}
          onToggleTheme={toggleTheme}
          onToggleSidebar={toggleSidebar}
          health={health}
        />
        <main className="flex-1 overflow-hidden relative">
          <div className="h-full overflow-auto p-4 md:p-6 has-[.log-viewer]:p-0 has-[.log-viewer]:overflow-hidden">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}

export function Layout() {
  return (
    <WsProvider>
      <LayoutInner />
    </WsProvider>
  )
}
