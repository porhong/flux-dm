import { Clock3, Download, FolderOpen, Settings2, type LucideIcon } from "lucide-react"

import type { AppSection } from "@/stores/ui-store"

export interface NavigationItem {
  id: AppSection
  icon: LucideIcon
  label: string
  description: string
}

export const navigation: NavigationItem[] = [
  { id: "downloads", icon: Download, label: "Downloads", description: "Your transfer workspace is ready." },
  { id: "categories", icon: FolderOpen, label: "Categories", description: "Organize downloads by file type and destination." },
  { id: "scheduler", icon: Clock3, label: "Scheduler", description: "Plan when queues start and stop." },
  { id: "settings", icon: Settings2, label: "Settings", description: "Configure the FluxDM desktop experience." },
]

export function getNavigationItem(section: AppSection): NavigationItem {
  return navigation.find((item) => item.id === section) ?? navigation[0]
}
