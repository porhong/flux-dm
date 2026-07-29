import { useCallback, useEffect, useState } from "react"

import { Events } from "@wailsio/runtime"
import { ConfirmDownloadDialog } from "./confirm-download-dialog"
import { hideBrowserConfirmation, isDownloadRequestEvent, listPendingBrowserDownloads, type DownloadRequestEvent } from "@/lib/backend"

// This surface lives in its own Wails window. It intentionally has no shell,
// navigation, or download history: browser handoffs should not disturb the
// dashboard the user was already using.
export function BrowserConfirmationSurface() {
  const [request, setRequest] = useState<DownloadRequestEvent | null>(null)
  const refresh = useCallback(async () => {
    const pending = await listPendingBrowserDownloads()
    setRequest(pending[0] ?? null)
    if (pending.length === 0) await hideBrowserConfirmation()
  }, [])

  useEffect(() => {
    const onRequested = (payload: unknown) => { if (isDownloadRequestEvent(payload)) void refresh() }
    Events.On("browser:handoff-pending", onRequested)
    Events.On("download:requested", onRequested)
    void Promise.resolve().then(refresh)
    return () => { Events.Off("browser:handoff-pending"); Events.Off("download:requested") }
  }, [refresh])

  return <ConfirmDownloadDialog key={request?.pendingId ?? "empty"} request={request} onClose={() => { void refresh() }} />
}
