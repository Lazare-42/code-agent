"use client"

import { CircleDot, Play, CheckCircle2, XCircle, Clock, FolderGit2 } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import type { Stats } from "@/lib/schemas/stats"

interface StatsCardsProps {
  stats: Stats | undefined
  isLoading: boolean
}

export function StatsCards({ stats, isLoading }: StatsCardsProps) {
  const cards = [
    { label: "Open Issues", value: stats?.open_issues ?? 0, icon: CircleDot },
    { label: "Total Issues", value: stats?.total_issues ?? 0, icon: CircleDot },
    { label: "Total Runs", value: stats?.total_runs ?? 0, icon: Play },
    {
      label: "Successful",
      value: stats?.runs_by_status?.success ?? 0,
      icon: CheckCircle2,
    },
    {
      label: "Failed",
      value: stats?.runs_by_status?.failed ?? 0,
      icon: XCircle,
    },
    { label: "Repos", value: stats?.repo_count ?? 0, icon: FolderGit2 },
  ]

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
      {cards.map((card) => (
        <Card key={card.label}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{card.label}</CardTitle>
            <card.icon className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-16" />
            ) : (
              <div className="text-2xl font-bold">{card.value}</div>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
