import { z } from "zod/v4"

export const paginationMetaSchema = z.object({
  total: z.number(),
  limit: z.number(),
  offset: z.number(),
})

export type PaginationMeta = z.infer<typeof paginationMetaSchema>

export function listResponseSchema<T extends z.ZodType>(dataSchema: T) {
  return z.object({
    data: z.array(dataSchema),
    meta: paginationMetaSchema,
  })
}

export function detailResponseSchema<T extends z.ZodType>(dataSchema: T) {
  return z.object({
    data: dataSchema,
  })
}
