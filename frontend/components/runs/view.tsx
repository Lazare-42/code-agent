"use client"

import { useState } from "react"
import { useRuns } from "@/hooks/use-runs"
import { PageHeading } from "@/components/layout/page-heading"
import { RunsTable } from "./table"
import type { RunFilters } from "@/lib/api-client"

export function RunsView() {
  const [filters, setFilters] = useState<RunFilters>({ limit: 20 })
  const { data, isLoading } = useRuns(filters)

  return (
    <>
      <PageHeading title="Agent Runs" description="Dispatch history and results" />
      <RunsTable
        data={data?.data ?? []}
        meta={data?.meta ?? { total: 0, limit: 20, offset: 0 }}
        isLoading={isLoading}
        onPageChange={(offset) => setFilters((f) => ({ ...f, offset }))}
      />
    </>
  )
}
