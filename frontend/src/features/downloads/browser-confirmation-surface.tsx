import { useCallback, useEffect, useRef, useState } from "react"
import { AlertCircle, LoaderCircle } from "lucide-react"

import { Events } from "@wailsio/runtime"
import { Button } from "@/components/ui/button"
import { ConfirmDownloadDialog } from "./confirm-download-dialog"
import { hideBrowserConfirmation, isDownloadRequestEvent, listPendingBrowserDownloads, type DownloadRequestEvent } from "@/lib/backend"

// This surface lives in its own Wails window. It intentionally has no shell,
// navigation, or download history: browser handoffs should not disturb the
// dashboard the user was already using.
export function BrowserConfirmationSurface() {
  const [request, setRequest] = useState<DownloadRequestEvent | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const latestRefresh = useRef(0)
  const isMounted = useRef(false)

  const refresh = useCallback(async () => {
    const refreshID = ++latestRefresh.current
    setLoading(true)
    setLoadError(null)
    try {
      const pending = await listPendingBrowserDownloads()
      if (refreshID !== latestRefresh.current || !isMounted.current) return
      const nextRequest = pending[0] ?? null
      setRequest(nextRequest)
      if (!nextRequest) await hideBrowserConfirmation()
    } catch (cause) {
      if (refreshID === latestRefresh.current && isMounted.current) {
        setRequest(null)
        setLoadError(errorMessage(cause))
      }
    } finally {
      if (refreshID === latestRefresh.current && isMounted.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    isMounted.current = true
    // The direct window event is the reliable signal for a newly opened
    // confirmation surface. Accept the legacy empty signal too: refresh() is
    // idempotent and the pending store remains the source of truth.
    const onRequested = (payload: unknown) => {
      if (payload === null || payload === undefined || isDownloadRequestEvent(payload)) void refresh()
    }
    Events.On("browser:handoff-pending", onRequested)
    Events.On("download:requested", onRequested)
    void Promise.resolve().then(refresh)
    return () => {
      isMounted.current = false
      Events.Off("browser:handoff-pending")
      Events.Off("download:requested")
    }
  }, [refresh])

  if (!request) {
    return <ConfirmationWindowStatus loading={loading} error={loadError} onRetry={() => void refresh()} onClose={() => void hideBrowserConfirmation()} />
  }

  return <ConfirmDownloadDialog key={request.pendingId} request={request} onClose={() => { void refresh() }} />
}

function ConfirmationWindowStatus({ loading, error, onRetry, onClose }: { loading: boolean; error: string | null; onRetry: () => void; onClose: () => void }) {
  if (loading) {
    return <main className="grid min-h-screen place-items-center bg-canvas p-6 text-center" aria-busy="true">
      <div className="space-y-3">
        <LoaderCircle className="mx-auto size-6 animate-spin text-cyan-300" aria-hidden="true" />
        <p className="text-sm font-medium text-slate-100" role="status">Preparing your download…</p>
        <p className="text-xs text-slate-500">Loading the download details.</p>
      </div>
    </main>
  }

  if (error) {
    return <main className="grid min-h-screen place-items-center bg-canvas p-6 text-center">
      <div className="max-w-sm space-y-4">
        <AlertCircle className="mx-auto size-7 text-red-300" aria-hidden="true" />
        <div><h1 className="text-base font-semibold text-slate-100">Could not open this download</h1><p className="mt-2 text-sm text-slate-400" role="alert">{error}</p></div>
        <div className="flex justify-center gap-2"><Button type="button" variant="outline" onClick={onClose}>Close</Button><Button type="button" onClick={onRetry}>Try again</Button></div>
      </div>
    </main>
  }

  return <main className="grid min-h-screen place-items-center bg-canvas p-6 text-center"><p className="text-sm text-slate-400" role="status">No download request is waiting.</p></main>
}

function errorMessage(cause: unknown): string {
  if (cause instanceof Error) return cause.message
  if (typeof cause === "string") return cause
  return "FluxDM could not load the browser download request."
}
