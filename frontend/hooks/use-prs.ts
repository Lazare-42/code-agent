"use client"

import { useQuery } from "@tanstack/react-query"
import { fetchPRs, fetchPRDetail } from "@/lib/api-client"

export function usePRs() {
  return useQuery({
    queryKey: ["prs"],
    queryFn: fetchPRs,
    refetchInterval: 60_000,
  })
}

export function usePRDetail(repo: string, number: number, enabled = true) {
  return useQuery({
    queryKey: ["pr-detail", repo, number],
    queryFn: () => fetchPRDetail(repo, number),
    enabled,
  })
}
