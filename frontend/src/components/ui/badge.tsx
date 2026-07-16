import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva("inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold", {
  variants: {
    variant: {
      default: "border-cyan-300/20 bg-cyan-300/10 text-cyan-200",
      secondary: "border-white/10 bg-white/5 text-slate-300",
      destructive: "border-red-400/20 bg-red-400/10 text-red-200",
      outline: "border-white/15 text-slate-200",
    },
  },
  defaultVariants: { variant: "default" },
})

function Badge({ className, variant, ...props }: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge }
