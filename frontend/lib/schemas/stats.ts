import { z } from "zod/v4"

export const statsSchema = z.object({
  total_issues: z.number(),
  open_issues: z.number(),
  total_runs: z.number(),
  runs_by_status: z.record(z.string(), z.number()),
  repo_count: z.number(),
})

export type Stats = z.infer<typeof statsSchema>
