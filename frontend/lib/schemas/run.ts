import { z } from "zod/v4"

export const runSchema = z.object({
  id: z.number(),
  repo: z.string(),
  issue_num: z.number(),
  branch: z.string(),
  status: z.string(),
  pr_url: z.string().optional(),
  error_msg: z.string().optional(),
  agent_output: z.string().optional(),
  review_verdict: z.string().nullable().optional(),
  review_comment: z.string().optional(),
  task_title: z.string().nullable().optional(),
  task_prompt: z.string().nullable().optional(),
  started_at: z.string(),
  ended_at: z.string().nullable().optional(),
  duration_s: z.number().nullable().optional(),
})

export type Run = z.infer<typeof runSchema>
