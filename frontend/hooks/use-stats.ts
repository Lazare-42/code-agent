"use client"

import { useQuery } from "@tanstack/react-query"
import { fetchStats } from "@/lib/api-client"

export function useStats() {
  return useQuery({
    queryKey: ["stats"],
    queryFn: fetchStats,
    refetchInterval: 60_000,
  })
}
