import { useEffect, useState } from "react"
import { Plus } from "lucide-react"
import { Events } from "@wailsio/runtime"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { getNavigationItem, navigation } from "@/features/shell/navigation"
import { SectionContent } from "@/features/shell/section-content"
import { healthCheck, type HealthStatus } from "@/lib/backend"
import { useUIStore } from "@/stores/ui-store"

interface ReadyEvent {
  name: string
  version: string
  message: string
}

function isReadyEvent(value: unknown): value is ReadyEvent {
  if (typeof value !== "object" || value === null) return false
  const event = value as Record<string, unknown>
  return typeof event.name === "string" && typeof event.version === "string" && typeof event.message === "string"
}

export default function App() {
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [readyMessage, setReadyMessage] = useState("Connecting to backend…")
  const [error, setError] = useState<string | null>(null)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const activeSection = useUIStore((state) => state.activeSection)
  const setActiveSection = useUIStore((state) => state.setActiveSection)
  const currentNavigation = getNavigationItem(activeSection)

  useEffect(() => {
    Events.On("app:ready", (payload: unknown) => {
      if (isReadyEvent(payload)) setReadyMessage(payload.message)
    })
    Events.On("tray:add-download", () => {
      setActiveSection("downloads")
      setAddDialogOpen(true)
    })
		Events.On("tray:updates", () => setActiveSection("settings"))
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "n") {
        event.preventDefault()
        setActiveSection("downloads")
        setAddDialogOpen(true)
      }
    }
    window.addEventListener("keydown", onKeyDown)
    void healthCheck()
      .then(setHealth)
      .catch(() => {
        setError("Backend health check failed")
        setReadyMessage("Backend unavailable")
      })
    return () => {
      Events.Off("app:ready")
      Events.Off("tray:add-download")
		Events.Off("tray:updates")
      window.removeEventListener("keydown", onKeyDown)
    }
  }, [setActiveSection])

  return (
    <TooltipProvider delayDuration={250}>
      <div className="matcha-shell flex min-h-screen bg-canvas text-foreground">
        <aside className="app-sidebar flex shrink-0 flex-col border-r border-border bg-sidebar/95 p-4">
          <div className="brand-lockup mb-8 px-2 pt-2">
            <img className="brand-mark brand-mark--full" src="/FluxDM-Logo-full.png" alt="FluxDM" />
            <img className="brand-mark brand-mark--compact" src="/FluxDM-Logo.png" alt="FluxDM" />
            <p className="brand-tagline">Windows download manager</p>
          </div>

          <nav className="space-y-1" aria-label="Main navigation">
            {navigation.map((item) => {
              const isActive = item.id === activeSection
              return (
                <button
                  key={item.id}
                  className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm outline-none transition focus-visible:ring-2 focus-visible:ring-ring ${isActive ? "bg-primary/10 text-cyan-200" : "text-slate-400 hover:bg-surface-raised hover:text-foreground"}`}
                  type="button"
                  aria-current={isActive ? "page" : undefined}
                  onClick={() => setActiveSection(item.id)}
                >
                  <item.icon className="size-4" />
                  <span className="sidebar-nav-label">{item.label}</span>
                </button>
              )
            })}
          </nav>

          <div className="mt-auto">
            <footer className="brand-credit" aria-label="Powered by BadbotDev">
              <span className="brand-credit-label">Powered by</span>
              <img className="brand-credit-logo brand-credit-logo--full" src="/BadbotDev-Logo.png" alt="BadbotDev" />
              <img className="brand-credit-logo brand-credit-logo--compact" src="/BadbotDev-Logo-symbol.png" alt="BadbotDev" />
            </footer>

            <div className="sidebar-status rounded-xl border border-border bg-surface p-3">
              <div className="mb-2 flex items-center justify-between text-xs">
                <span className="sidebar-status-label text-slate-400">Backend</span>
                <Badge variant={error ? "destructive" : health ? "default" : "secondary"}>
                  {error ? "Offline" : health ? "Healthy" : "Checking"}
                </Badge>
              </div>
              <p className="sidebar-status-message truncate text-xs text-slate-500" title={readyMessage}>{readyMessage}</p>
            </div>
          </div>
        </aside>

        <main className="min-w-0 flex-1 overflow-hidden">
          <header className="flex h-20 items-center justify-between border-b border-border px-8">
            <div>
              <h1 className="text-xl font-semibold tracking-tight">{currentNavigation.label}</h1>
              <p className="mt-1 text-sm text-slate-500">{currentNavigation.description}</p>
            </div>
            {activeSection === "downloads" && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button aria-label="Add download" onClick={() => setAddDialogOpen(true)}><Plus className="size-4" /> Add download</Button>
                </TooltipTrigger>
                <TooltipContent>Add an HTTP or HTTPS download</TooltipContent>
              </Tooltip>
            )}
          </header>

          <section className="h-[calc(100vh-5rem)] overflow-auto p-8">
            <SectionContent section={activeSection} health={health} hasBackendError={error !== null} addDialogOpen={addDialogOpen} onAddDialogOpenChange={setAddDialogOpen} />
          </section>
        </main>
      </div>
    </TooltipProvider>
  )
}
