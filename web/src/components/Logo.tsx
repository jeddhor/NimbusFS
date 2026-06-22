import { cn } from "@/lib/utils"

export function Logo({ size = 22, className }: { size?: number; className?: string }) {
  return <img src="/favicon.svg" alt="" width={size} height={size} className={cn("flex-shrink-0", className)} />
}

export function Wordmark({ className }: { className?: string }) {
  return <span className={cn("brand-wordmark", className)}>NimbusFS</span>
}
