import { create } from "zustand"

type Density = "comfortable" | "compact"
export type AppSection = "downloads" | "categories" | "scheduler" | "settings"

interface UIState {
  activeSection: AppSection
  density: Density
  setActiveSection: (section: AppSection) => void
  setDensity: (density: Density) => void
}

export const useUIStore = create<UIState>((set) => ({
  activeSection: "downloads",
  density: "comfortable",
  setActiveSection: (activeSection) => set({ activeSection }),
  setDensity: (density) => set({ density }),
}))
