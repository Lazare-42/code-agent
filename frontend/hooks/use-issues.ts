"use client"

import { useQuery } from "@tanstack/react-query"
import { fetchIssues, type IssueFilters } from "@/lib/api-client"

export function useIssues(filters: IssueFilters = {}) {
  return useQuery({
    queryKey: ["issues", filters],
    queryFn: () => fetchIssues(filters),
  })
}
