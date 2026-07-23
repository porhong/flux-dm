import { useEffect, useState } from "react"
import { ArrowDownToLine, Plus } from "lucide-react"
import { EventsOff, EventsOn } from "../wailsjs/runtime/runtime"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { ConfirmDownloadDialog } from "@/features/downloads/confirm-download-dialog"
import { getNavigationItem, navigation } from "@/features/shell/navigation"
import { SectionContent } from "@/features/shell/section-content"
import { healthCheck, isDownloadRequestEvent, listPendingBrowserDownloads, type DownloadRequestEvent, type HealthStatus } from "@/lib/backend"
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
  const [confirmQueue, setConfirmQueue] = useState<DownloadRequestEvent[]>([])
  const activeSection = useUIStore((state) => state.activeSection)
  const setActiveSection = useUIStore((state) => state.setActiveSection)
  const currentNavigation = getNavigationItem(activeSection)

  useEffect(() => {
    const enqueueBrowserRequests = (requests: DownloadRequestEvent[]) => {
      setConfirmQueue((queue) => {
        const pendingIDs = new Set(queue.map((request) => request.pendingId))
        const unseen = requests.filter((request) => !pendingIDs.has(request.pendingId))
        return unseen.length === 0 ? queue : [...queue, ...unseen]
      })
    }
    EventsOn("app:ready", (payload: unknown) => {
      if (isReadyEvent(payload)) setReadyMessage(payload.message)
    })
    EventsOn("tray:add-download", () => {
      setActiveSection("downloads")
      setAddDialogOpen(true)
    })
    EventsOn("download:requested", (payload: unknown) => {
      if (isDownloadRequestEvent(payload)) {
        setActiveSection("downloads")
        enqueueBrowserRequests([payload])
      }
    })
    // A native handoff can start FluxDM before React registers the event
    // listener. Recover those parked requests after subscribing so an event
    // that arrives during this call is merged rather than lost.
    void listPendingBrowserDownloads().then((requests) => {
      if (requests.length > 0) setActiveSection("downloads")
      enqueueBrowserRequests(requests)
    })
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
      EventsOff("app:ready")
      EventsOff("tray:add-download")
      EventsOff("download:requested")
      window.removeEventListener("keydown", onKeyDown)
    }
  }, [setActiveSection])

  return (
    <TooltipProvider delayDuration={250}>
      <div className="matcha-shell flex min-h-screen bg-canvas text-foreground">
        <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-sidebar/95 p-4">
          <div className="mb-8 flex items-center gap-3 px-2 pt-2">
            <div className="grid size-10 place-items-center rounded-xl bg-primary text-primary-foreground shadow-lg shadow-primary/15">
              <ArrowDownToLine className="size-5" strokeWidth={2.5} />
            </div>
            <div>
              <div className="text-lg font-bold tracking-tight">FluxDM</div>
              <div className="text-xs text-slate-500">Windows download manager</div>
            </div>
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
                  {item.label}
                </button>
              )
            })}
          </nav>

          <div className="mt-auto rounded-xl border border-border bg-surface p-3">
            <div className="mb-2 flex items-center justify-between text-xs">
              <span className="text-slate-400">Backend</span>
              <Badge variant={error ? "destructive" : health ? "default" : "secondary"}>
                {error ? "Offline" : health ? "Healthy" : "Checking"}
              </Badge>
            </div>
            <p className="truncate text-xs text-slate-500" title={readyMessage}>{readyMessage}</p>
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

        <ConfirmDownloadDialog
          key={confirmQueue[0]?.pendingId ?? "empty"}
          request={confirmQueue[0] ?? null}
          onClose={() => setConfirmQueue((queue) => queue.slice(1))}
        />
      </div>
    </TooltipProvider>
  )
}
