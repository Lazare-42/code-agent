"use client"

import { useStats } from "@/hooks/use-stats"
import { useRuns } from "@/hooks/use-runs"
import { PageHeading } from "@/components/layout/page-heading"
import { StatsCards } from "./stats-cards"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { RUN_STATUS_COLORS } from "@/lib/constants"

export function DashboardView() {
  const { data: stats, isLoading: statsLoading } = useStats()
  const { data: recentRuns, isLoading: runsLoading } = useRuns({ limit: 10 })

  return (
    <>
      <PageHeading title="Dashboard" description="Agent overview" />
      <div className="space-y-6">
        <StatsCards stats={stats} isLoading={statsLoading} />
        <Card>
          <CardHeader>
            <CardTitle>Recent Runs</CardTitle>
          </CardHeader>
          <CardContent>
            {runsLoading ? (
              <div className="space-y-3">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : !recentRuns?.data.length ? (
              <p className="text-sm text-muted-foreground">
                No agent runs yet.
              </p>
            ) : (
              <div className="space-y-3">
                {recentRuns.data.map((run) => (
                  <div
                    key={run.id}
                    className="flex items-center justify-between rounded-md border px-3 py-2"
                  >
                    <div className="min-w-0">
                      <p className="font-medium truncate">
                        {run.repo} #{run.issue_num}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {run.branch}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0 ml-3">
                      <Badge
                        variant="secondary"
                        className={
                          RUN_STATUS_COLORS[run.status] ?? ""
                        }
                      >
                        {run.status}
                      </Badge>
                      {run.pr_url && (
                        <a
                          href={run.pr_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-xs text-blue-500 hover:underline"
                        >
                          PR
                        </a>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  )
}
