import { z } from "zod/v4"

export const issueSchema = z.object({
  id: z.number(),
  repo: z.string(),
  number: z.number(),
  title: z.string(),
  body: z.string().optional(),
  labels: z.array(z.string()).nullable().optional(),
  state: z.string(),
  url: z.string().optional(),
  repo_url: z.string().optional(),
  first_seen: z.string(),
  updated_at: z.string(),
})

export type Issue = z.infer<typeof issueSchema>
