import { useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { FolderOpen, LoaderCircle, Search } from "lucide-react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { createDownload, probeURL, selectDestinationDirectory, startDownload, type DownloadItem, type ProbeResult } from "@/lib/backend"

const formSchema = z.object({
  url: z.string().trim().url("Enter a valid HTTP or HTTPS URL.").refine((value) => value.startsWith("http://") || value.startsWith("https://"), "Only HTTP and HTTPS URLs are supported."),
  destinationDir: z.string().trim().min(1, "Choose a destination folder."),
  fileName: z.string().trim().max(240, "The filename is too long."),
  connections: z.union([z.literal(1), z.literal(2), z.literal(4), z.literal(8), z.literal(16)]),
  bandwidthLimitMiB: z.number().min(0).max(10240),
	confirmExecutable:z.boolean(),
})

type FormValues = z.infer<typeof formSchema>

interface AddDownloadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (download: DownloadItem) => void
}

export function AddDownloadDialog({ open, onOpenChange, onCreated }: AddDownloadDialogProps) {
  const [probe, setProbe] = useState<ProbeResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [probing, setProbing] = useState(false)
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { url: "", destinationDir: "", fileName: "", connections: 4, bandwidthLimitMiB: 0, confirmExecutable:false },
  })
  const selectedConnections = useWatch({ control: form.control, name: "connections" })

  const inspectURL = async () => {
    const valid = await form.trigger("url")
    if (!valid) return null
    setError(null)
    setProbing(true)
    try {
      const result = await probeURL(form.getValues("url"))
      setProbe(result)
      if (!form.getValues("fileName")) form.setValue("fileName", result.fileName, { shouldValidate: true })
      return result
    } catch (cause) {
      setError(errorMessage(cause))
      return null
    } finally {
      setProbing(false)
    }
  }

  const chooseDirectory = async () => {
    setError(null)
    try {
      const directory = await selectDestinationDirectory()
      if (directory) form.setValue("destinationDir", directory, { shouldValidate: true })
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }

  const submit = form.handleSubmit(async (values) => {
    setError(null)
    const inspected = probe?.url === values.url ? probe : await inspectURL()
    if (!inspected) return
	if(inspected.executableWarning&&!values.confirmExecutable){setError("Confirm that you want to download this executable or script.");return}
    try {
      const created = await createDownload({
        url: values.url,
        destinationDir: values.destinationDir,
        fileName: values.fileName || inspected.fileName,
        connections: values.connections,
        bandwidthLimit: Math.round(values.bandwidthLimitMiB * 1024 * 1024),
		confirmExecutable:values.confirmExecutable,
      })
      onCreated(created)
      await startDownload(created.id)
      form.reset()
      setProbe(null)
      onOpenChange(false)
    } catch (cause) {
      setError(errorMessage(cause))
    }
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add download</DialogTitle>
          <DialogDescription>FluxDM will inspect the URL before adding it to the transfer queue.</DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={submit}>
          <Field label="Download URL" error={form.formState.errors.url?.message}>
            <div className="flex gap-2">
              <Input aria-label="Download URL" placeholder="https://example.com/file.zip" autoFocus {...form.register("url")} onChange={(event) => { setProbe(null); form.register("url").onChange(event) }} />
              <Button type="button" variant="outline" onClick={() => void inspectURL()} disabled={probing}>
                {probing ? <LoaderCircle className="size-4 animate-spin" /> : <Search className="size-4" />} Inspect
              </Button>
            </div>
          </Field>

          <Field label="Speed limit (MiB/s, 0 = unlimited)" error={form.formState.errors.bandwidthLimitMiB?.message}>
            <Input aria-label="Speed limit" type="number" min={0} max={10240} step="0.25" {...form.register("bandwidthLimitMiB", { valueAsNumber: true })} />
          </Field>

          <Field label="Destination folder" error={form.formState.errors.destinationDir?.message}>
            <div className="flex gap-2">
              <Input aria-label="Destination folder" placeholder="C:\\Users\\you\\Downloads" {...form.register("destinationDir")} />
              <Button type="button" variant="outline" onClick={() => void chooseDirectory()}><FolderOpen className="size-4" /> Browse</Button>
            </div>
          </Field>

          <Field label="Filename" error={form.formState.errors.fileName?.message}>
            <Input aria-label="Filename" placeholder="Detected automatically" {...form.register("fileName")} />
          </Field>

          <Field label="Connections" error={form.formState.errors.connections?.message}>
            <Controller
              control={form.control}
              name="connections"
              render={({ field }) => (
                <Select value={String(field.value)} onValueChange={(value) => field.onChange(Number(value))}>
                  <SelectTrigger aria-label="Connections"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {[1, 2, 4, 8, 16].map((value) => <SelectItem key={value} value={String(value)}>{value} {value === 1 ? "connection" : "connections"}</SelectItem>)}
                  </SelectContent>
                </Select>
              )}
            />
          </Field>

          {probe && (
            <div className="rounded-lg border border-cyan-300/15 bg-cyan-300/5 p-3 text-xs text-slate-400">
              <span className="font-medium text-cyan-200">URL verified</span>
              <span className="ml-2">{formatBytes(probe.totalBytes)} · {probe.mimeType || "Unknown type"} · {probe.rangeSupported ? `${selectedConnections} ranged connections` : "Single-stream fallback"}</span>
            </div>
          )}
		  {probe?.executableWarning&&<label className="flex items-start gap-2 rounded-lg border border-amber-400/20 bg-amber-400/5 p-3 text-sm text-amber-100"><input type="checkbox" {...form.register("confirmExecutable")}/><span>This file can run code on Windows. I chose this source and want FluxDM to download it. FluxDM will not open it automatically.</span></label>}
          {error && <p className="rounded-lg border border-red-400/15 bg-red-400/5 p-3 text-sm text-red-200" role="alert">{error}</p>}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={form.formState.isSubmitting || probing}>
              {form.formState.isSubmitting && <LoaderCircle className="size-4 animate-spin" />} Add and start
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium text-slate-200">{label}</span>
      {children}
      {error && <span className="block text-xs text-red-300">{error}</span>}
    </label>
  )
}

function errorMessage(cause: unknown): string {
  if (cause instanceof Error) return cause.message
  if (typeof cause === "string") return cause
  return "FluxDM could not complete that action."
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
