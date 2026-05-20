import { z } from "zod/v4"

export const repoSchema = z.object({
  name: z.string(),
  path: z.string(),
  remote: z.string().optional(),
  enabled: z.boolean(),
  last_poll_at: z.string().nullable().optional(),
})

export type Repo = z.infer<typeof repoSchema>
