import { useCallback, useEffect, useRef, useState } from "react"
import { useNavigate } from "react-router-dom"
import { LayoutDashboard, ScrollText, Server, Bell, Settings, Search } from "lucide-react"
import { cn } from "@/lib/utils"

const commands = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard, path: "/", shortcut: "D" },
  { id: "logs", label: "Logs", icon: ScrollText, path: "/logs", shortcut: "L" },
  { id: "sources", label: "Sources", icon: Server, path: "/sources", shortcut: "S" },
  { id: "alerts", label: "Alerts", icon: Bell, path: "/alerts", shortcut: "A" },
  { id: "config", label: "Configuration", icon: Settings, path: "/config", shortcut: "" },
]

export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [selected, setSelected] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  const filtered = query
    ? commands.filter((c) => c.label.toLowerCase().includes(query.toLowerCase()))
    : commands

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault()
        setOpen((prev) => !prev)
        setQuery("")
        setSelected(0)
      }
      if (e.key === "Escape" && open) {
        setOpen(false)
      }
    }
    window.addEventListener("keydown", handler)
    return () => window.removeEventListener("keydown", handler)
  }, [open])

  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 50)
  }, [open])

  const run = useCallback((path: string) => {
    navigate(path)
    setOpen(false)
  }, [navigate])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setSelected((i) => Math.min(filtered.length - 1, i + 1))
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setSelected((i) => Math.max(0, i - 1))
    } else if (e.key === "Enter" && filtered[selected]) {
      run(filtered[selected].path)
    }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh] bg-black/50" onClick={() => setOpen(false)}>
      <div
        className="bg-surface border border-border rounded-lg shadow-2xl w-full max-w-md mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Search className="h-4 w-4 text-text-secondary shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => { setQuery(e.target.value); setSelected(0) }}
            onKeyDown={handleKeyDown}
            placeholder="Type a command..."
            className="flex-1 bg-transparent text-sm text-text-primary placeholder:text-text-secondary focus:outline-none"
          />
          <kbd className="hidden sm:inline-block text-[10px] font-mono text-text-secondary bg-surface-hover border border-border rounded px-1.5 py-0.5">
            ESC
          </kbd>
        </div>

        <div className="py-1 max-h-64 overflow-y-auto">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-sm text-text-secondary">No results</div>
          ) : (
            filtered.map((cmd, i) => {
              const Icon = cmd.icon
              return (
                <button
                  key={cmd.id}
                  onClick={() => run(cmd.path)}
                  onMouseEnter={() => setSelected(i)}
                  className={cn(
                    "flex items-center gap-3 w-full px-4 py-2.5 text-left text-sm transition-colors",
                    i === selected
                      ? "bg-accent/10 text-accent"
                      : "text-text-secondary hover:text-text-primary"
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  <span className="flex-1">{cmd.label}</span>
                  {cmd.shortcut && (
                    <kbd className="text-[10px] font-mono text-text-secondary bg-surface-hover border border-border rounded px-1.5 py-0.5">
                      {cmd.shortcut}
                    </kbd>
                  )}
                </button>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
