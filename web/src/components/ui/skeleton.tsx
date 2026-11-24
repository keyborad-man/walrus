import { cn } from '@/lib/utils'

export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={cn('rounded-md dst-skeleton', className)} />
}

export function DashboardSkeleton() {
  return (
    <div className="rounded-[10px] surface-card divide-y divide-border" aria-busy="true">
      {[0, 1].map((idx) => (
        <div key={idx} className="p-4 flex items-center justify-between gap-4">
          <div className="space-y-2 flex-1">
            <Skeleton className="h-4 w-36" />
            <Skeleton className="h-3 w-24" />
          </div>
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-28 hidden md:block" />
          <Skeleton className="h-4 w-20 hidden lg:block" />
          <Skeleton className="h-7 w-20 rounded-md" />
        </div>
      ))}
    </div>
  )
}

export function ClusterOverviewSkeleton() {
  return (
    <div className="space-y-6" aria-busy="true">
      <div className="flex items-center justify-between gap-4">
        <div className="space-y-2">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-3.5 w-64" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-8 w-24 rounded-lg" />
          <Skeleton className="h-8 w-20 rounded-lg" />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <div className="lg:col-span-5 rounded-[10px] surface-card p-5 flex items-center gap-5">
          <Skeleton className="h-20 w-20 rounded-full shrink-0" />
          <div className="space-y-2 flex-1">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-3 w-40" />
          </div>
        </div>
        <div className="lg:col-span-7 rounded-[10px] surface-card p-5 space-y-4">
          <Skeleton className="h-3.5 w-36" />
          <Skeleton className="h-16 w-full rounded-lg" />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {[0, 1].map((idx) => (
          <div key={idx} className="rounded-[10px] surface-card p-5 space-y-4">
            <div className="flex justify-between">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-4 w-16" />
            </div>
            <Skeleton className="h-14 w-full rounded-lg" />
            <div className="flex gap-2">
              <Skeleton className="h-7 w-18 rounded-md" />
              <Skeleton className="h-7 w-16 rounded-md" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export function PageCardsSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className="space-y-4" aria-busy="true">
      {Array.from({ length: count }).map((_, idx) => (
        <div key={idx} className="rounded-[10px] surface-card p-5 space-y-4">
          <div className="space-y-2">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-3 w-64" />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Skeleton className="h-9 w-full rounded-lg" />
            <Skeleton className="h-9 w-full rounded-lg" />
          </div>
          <Skeleton className="h-9 w-full rounded-lg" />
        </div>
      ))}
    </div>
  )
}

export function TableRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-3 py-2" aria-busy="true">
      {Array.from({ length: rows }).map((_, idx) => (
        <div key={idx} className="flex items-center justify-between gap-4 py-2 border-b border-border/60 last:border-b-0">
          <div className="flex items-center gap-3 flex-1 min-w-0">
            <Skeleton className="h-7 w-7 rounded-md shrink-0" />
            <div className="space-y-1.5 flex-1">
              <Skeleton className="h-3.5 w-44 max-w-full" />
              <Skeleton className="h-3 w-28 max-w-full" />
            </div>
          </div>
          <Skeleton className="h-5 w-16 rounded-md shrink-0" />
        </div>
      ))}
    </div>
  )
}
