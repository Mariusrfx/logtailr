import { useState } from "react"
import { Settings, Upload } from "lucide-react"
import { cn } from "@/lib/utils"
import { SourcesTab } from "./SourcesTab"
import { OutputsTab } from "./OutputsTab"
import { AlertRulesTab } from "./AlertRulesTab"
import { SettingsTab } from "./SettingsTab"
import { ImportYamlModal } from "./ImportYamlModal"

const tabs = ["Sources", "Outputs", "Alert Rules", "Settings"] as const
type Tab = (typeof tabs)[number]

export function ConfigPage() {
  const [activeTab, setActiveTab] = useState<Tab>("Sources")
  const [showImport, setShowImport] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  const refresh = () => setRefreshKey((k) => k + 1)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Settings className="h-6 w-6 text-accent" />
          <h1 className="text-2xl font-bold text-text-primary">Configuration</h1>
        </div>
        <button
          onClick={() => setShowImport(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md border border-border text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
        >
          <Upload className="h-4 w-4" />
          Import YAML
        </button>
      </div>

      {/* Tabs */}
      <div className="border-b border-border">
        <nav className="flex gap-0 -mb-px">
          {tabs.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={cn(
                "px-4 py-2.5 text-sm font-medium border-b-2 transition-colors",
                activeTab === tab
                  ? "border-accent text-accent"
                  : "border-transparent text-text-secondary hover:text-text-primary hover:border-border"
              )}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab content */}
      <div>
        {activeTab === "Sources" && <SourcesTab refreshKey={refreshKey} />}
        {activeTab === "Outputs" && <OutputsTab refreshKey={refreshKey} />}
        {activeTab === "Alert Rules" && <AlertRulesTab refreshKey={refreshKey} />}
        {activeTab === "Settings" && <SettingsTab refreshKey={refreshKey} />}
      </div>

      {/* Import modal */}
      {showImport && (
        <ImportYamlModal
          onClose={() => setShowImport(false)}
          onImported={refresh}
        />
      )}
    </div>
  )
}
