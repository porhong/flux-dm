import { useSyncExternalStore } from "react"

import type { DownloadProgress } from "@/lib/backend"

type Listener = () => void

class ProgressSignal {
  private value: DownloadProgress | null = null
  private readonly listeners = new Set<Listener>()

  readonly subscribe = (listener: Listener) => {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  readonly getSnapshot = () => this.value

  publish(progress: DownloadProgress) {
    this.value = progress
    this.listeners.forEach((listener) => listener())
  }
}

const signals = new Map<string, ProgressSignal>()

function signalFor(id: string): ProgressSignal {
  let signal = signals.get(id)
  if (!signal) {
    signal = new ProgressSignal()
    signals.set(id, signal)
  }
  return signal
}

export function publishProgress(progress: DownloadProgress) {
  signalFor(progress.id).publish(progress)
}

export function useDownloadProgress(id: string) {
  const signal = signalFor(id)
  return useSyncExternalStore(signal.subscribe, signal.getSnapshot, signal.getSnapshot)
}
