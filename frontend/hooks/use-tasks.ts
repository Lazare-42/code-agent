"use client"

import { useQuery } from "@tanstack/react-query"
import { fetchTasks, type RunFilters } from "@/lib/api-client"

export function useTasks(filters: RunFilters = {}) {
  const query = useQuery({
    queryKey: ["tasks", filters],
    queryFn: () => fetchTasks(filters),
  })

  const hasRunning = query.data?.data.some((r) => r.status === "running")

  return useQuery({
    queryKey: ["tasks", filters],
    queryFn: () => fetchTasks(filters),
    refetchInterval: hasRunning ? 5000 : false,
  })
}
