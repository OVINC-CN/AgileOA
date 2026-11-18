import { z } from 'zod';

const workTargetSchema = z.object({
  id: z
    .string()
    .regex(/^[1-9]\d*$/u)
    .transform(Number)
    .refine(Number.isSafeInteger),
  year: z.string().regex(/^(?:19\d{2}|[2-9]\d{3})$/u),
  month: z
    .string()
    .regex(/^(?:0?[1-9]|1[0-2])$/u)
    .transform((value) => String(Number(value))),
});

export type WorkTarget = z.infer<typeof workTargetSchema>;

export function readWorkTarget(params: URLSearchParams): WorkTarget | undefined {
  const result = workTargetSchema.safeParse({
    id: params.get('work_id'),
    year: params.get('year'),
    month: params.get('month'),
  });
  return result.success ? result.data : undefined;
}
