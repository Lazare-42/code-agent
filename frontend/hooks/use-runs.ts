"use client"

import { useQuery } from "@tanstack/react-query"
import { fetchRuns, type RunFilters } from "@/lib/api-client"

export function useRuns(filters: RunFilters = {}) {
  const query = useQuery({
    queryKey: ["runs", filters],
    queryFn: () => fetchRuns(filters),
  })

  const hasRunning = query.data?.data.some((r) => r.status === "running")

  return useQuery({
    queryKey: ["runs", filters],
    queryFn: () => fetchRuns(filters),
    refetchInterval: hasRunning ? 5000 : false,
  })
}
