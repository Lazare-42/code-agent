"use client"

import { Select } from "@/components/ui/select"
import { useRepos } from "@/hooks/use-repos"
import type { IssueFilters } from "@/lib/api-client"

interface IssueFilterBarProps {
  filters: IssueFilters
  onChange: (filters: IssueFilters) => void
}

export function IssueFilterBar({ filters, onChange }: IssueFilterBarProps) {
  const { data: repos } = useRepos()

  return (
    <div className="flex gap-3 mb-4">
      <Select
        value={filters.repo ?? ""}
        onChange={(e) =>
          onChange({ ...filters, repo: e.target.value || undefined, offset: 0 })
        }
        className="w-48"
      >
        <option value="">All repos</option>
        {repos?.data.map((r) => (
          <option key={r.name} value={r.name}>
            {r.name}
          </option>
        ))}
      </Select>
    </div>
  )
}
