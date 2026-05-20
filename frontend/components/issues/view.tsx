"use client"

import { useState } from "react"
import { useIssues } from "@/hooks/use-issues"
import { PageHeading } from "@/components/layout/page-heading"
import { IssueFilterBar } from "./filters"
import { IssuesTable } from "./table"
import { CreateIssueDialog } from "./create-dialog"
import type { IssueFilters } from "@/lib/api-client"

export function IssuesView() {
  const [filters, setFilters] = useState<IssueFilters>({ limit: 50 })
  const { data, isLoading } = useIssues(filters)

  return (
    <>
      <div className="flex items-center justify-between mb-6">
        <PageHeading
          title="Issues"
          description="Tracked GitHub issues across repos"
        />
        <CreateIssueDialog />
      </div>
      <IssueFilterBar filters={filters} onChange={setFilters} />
      <IssuesTable
        data={data?.data ?? []}
        meta={data?.meta ?? { total: 0, limit: 50, offset: 0 }}
        isLoading={isLoading}
        onPageChange={(offset) => setFilters((f) => ({ ...f, offset }))}
      />
    </>
  )
}
