import { useEffect, useState } from "react"
import { FileDown, FolderOpen, LoaderCircle } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  confirmBrowserDownload,
  defaultDownloadDirectory,
  discardBrowserDownload,
  probeURL,
  selectDestinationDirectory,
  startDownload,
  type DownloadRequestEvent,
  type ProbeResult,
} from "@/lib/backend"

interface ConfirmDownloadDialogProps {
  request: DownloadRequestEvent | null
  onClose: () => void
}

export function ConfirmDownloadDialog({ request, onClose }: ConfirmDownloadDialogProps) {
  // The parent remounts this component via a key whenever the pending ID
  // changes, so initial state can be derived directly from the request.
  const [destinationDir, setDestinationDir] = useState("")
  const [fileName, setFileName] = useState(() => request?.suggestedFilename ?? "")
  const [probe, setProbe] = useState<ProbeResult | null>(null)
  const [probing, setProbing] = useState(() => request !== null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [confirmExecutable, setConfirmExecutable] = useState(false)
  // A resolved request has either been discarded or converted to a download.
  // Keeping this local guard prevents duplicate confirmation calls.
  const [resolved, setResolved] = useState(false)

  useEffect(() => {
    if (!request) return
    let active = true
    void defaultDownloadDirectory()
      .then((dir) => { if (active) setDestinationDir(dir) })
      .catch((cause) => { if (active) setError(errorMessage(cause)) })
    void probeURL(request.url)
      .then((result) => {
        if (!active) return
        setProbe(result)
        if (!request.suggestedFilename) setFileName(result.fileName)
      })
      .catch((cause) => { if (active) setError(errorMessage(cause)) })
      .finally(() => { if (active) setProbing(false) })
    return () => { active = false }
  }, [request])

  const chooseDirectory = async () => {
    setError(null)
    try {
      const dir = await selectDestinationDirectory()
      if (dir) setDestinationDir(dir)
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }

  const start = async () => {
    if (!request || resolved || !probe) return
    if (!destinationDir.trim()) {
      setError("Choose a destination folder.")
      return
    }
    if (probe.executableWarning && !confirmExecutable) {
      setError("Confirm that you want to download this executable or script.")
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      const created = await confirmBrowserDownload(
        request.pendingId,
        destinationDir.trim(),
        fileName.trim() || request.suggestedFilename,
        4,
        confirmExecutable,
      )
      setResolved(true)
      try {
        await startDownload(created.id)
        onClose()
      } catch {
        setError("The download was added but could not start. You can retry it from the downloads list.")
      }
    } catch (cause) {
      setError(errorMessage(cause))
    } finally {
      setSubmitting(false)
    }
  }

  const cancel = () => {
    if (request && !resolved) {
      setResolved(true)
      void discardBrowserDownload(request.pendingId)
    }
    onClose()
  }

  const source = sourceHost(probe?.finalUrl || request?.url || "")
  const executableWarning = probe?.executableWarning === true
  const canStart = !submitting && !probing && probe !== null && destinationDir.trim() !== "" && (!executableWarning || confirmExecutable) && !resolved

  return (
    <Dialog open={request !== null} onOpenChange={(next) => { if (!next) cancel() }}>
      <DialogContent className="max-w-md gap-5 p-5">
        <DialogHeader>
          <DialogTitle>Start this download?</DialogTitle>
          <DialogDescription>Choose where to save it, then FluxDM will add it to the transfer queue.</DialogDescription>
        </DialogHeader>

        <div className="min-w-0 space-y-4">
          <section className="flex w-full min-w-0 max-w-full items-center gap-3 rounded-xl border border-cyan-300/15 bg-cyan-300/5 p-3" aria-label="Browser download details">
            <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-cyan-300/10 text-cyan-200"><FileDown className="size-4" /></div>
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-slate-100" title={fileName || "Download"}>{fileName || "Download"}</p>
              <p className="truncate text-xs text-slate-400" title={source}>{source || "Unknown source"}</p>
            </div>
            <div className="ml-auto shrink-0 text-right text-xs text-slate-400">
              <p>{probing ? "Inspecting…" : probe ? formatBytes(probe.totalBytes) : "Unavailable"}</p>
              {!probing && probe?.mimeType ? <p className="mt-0.5 max-w-24 truncate" title={probe.mimeType}>{probe.mimeType}</p> : null}
            </div>
          </section>

          <div className="space-y-2">
            <div className="flex min-w-0 items-center justify-between gap-3">
              <label className="text-sm font-medium text-slate-200" htmlFor="browser-download-destination">Save to</label>
              <span className="text-xs text-slate-500">Defaults to Downloads</span>
            </div>
            <div className="flex min-w-0 gap-2">
              <Input className="min-w-0 flex-1" id="browser-download-destination" aria-label="Destination folder" value={destinationDir} onChange={(event) => setDestinationDir(event.target.value)} autoFocus />
              <Button className="shrink-0" type="button" variant="outline" aria-label="Browse destination folder" onClick={() => void chooseDirectory()} disabled={submitting}><FolderOpen className="size-4" /><span className="sr-only">Browse destination folder</span></Button>
            </div>
          </div>

          {executableWarning ? (
            <label className="flex items-start gap-2 rounded-lg border border-amber-400/20 bg-amber-400/5 p-3 text-sm text-amber-100">
              <input type="checkbox" checked={confirmExecutable} onChange={(event) => setConfirmExecutable(event.target.checked)} disabled={submitting} />
              <span>This file can run code on Windows. I chose this source and want FluxDM to download it. FluxDM will not open it automatically.</span>
            </label>
          ) : null}

          {error ? <p className="rounded-lg border border-red-400/15 bg-red-400/5 p-3 text-sm text-red-200" role="alert">{error}</p> : null}
        </div>

        <DialogFooter className="min-w-0">
          <Button type="button" variant="ghost" onClick={cancel} disabled={submitting}>{resolved ? "Close" : "Cancel"}</Button>
          <Button type="button" onClick={() => void start()} disabled={!canStart}>
            {(submitting || probing) && <LoaderCircle className="size-4 animate-spin" />} Start download
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function errorMessage(cause: unknown): string {
  if (cause instanceof Error) return cause.message
  if (typeof cause === "string") return cause
  return "FluxDM could not complete that action."
}

function sourceHost(rawURL: string): string {
  try {
    return new URL(rawURL).host
  } catch {
    return ""
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 0) return "Unknown size"
  if (bytes < 1024) return `${bytes} B`
  const units = ["KB", "MB", "GB", "TB"]
  let value = bytes / 1024
  let unit = units[0]
  for (let index = 1; value >= 1024 && index < units.length; index++) {
    value /= 1024
    unit = units[index]
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} ${unit}`
}
