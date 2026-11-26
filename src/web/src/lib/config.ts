import { z } from 'zod';

import { deploymentFields } from '@/lib/deployment-config';

const contentTypeSchema = z
  .string()
  .regex(/^[a-z0-9][a-z0-9!#$&^_.+-]*\/(?:[a-z0-9][a-z0-9!#$&^_.+-]*|\*)$/u);

const pollIntervalSchema = z
  .object({
    min: z.number().int().positive(),
    max: z.number().int().positive(),
  })
  .refine(({ min, max }) => min <= max, { message: '刷新间隔上限不能小于下限' });

const configSchema = z
  .object({
    displayTimeZone: z.string().min(1),
    appName: z.string().trim().min(1),
    copyrightOwner: z.string().trim().min(1),
    copyrightYear: z.number().int().positive(),
    passwordLoginEnabled: z.boolean(),
    ...deploymentFields,
    aegis: z
      .object({
        id: z.string().trim().default(''),
        hostUrl: z.url({ protocol: /^https$/u }),
        aid: z.literal(false),
        device: z.literal(false),
        spa: z.boolean(),
        reportApiSpeed: z.boolean(),
        reportAssetSpeed: z.boolean(),
        api: z.object({ apiDetail: z.literal(false) }),
      })
      .optional(),
    apiTimeoutMs: z.number().int().positive(),
    messageDurationMs: z.number().int().positive(),
    pageUpdateCheckIntervalMs: pollIntervalSchema,
    pageUpdateCheckTimeoutMs: z.number().int().positive(),
    queryStaleTimeMs: z.number().int().nonnegative(),
    queryRetryCount: z.number().int().nonnegative(),
    notificationPollIntervalMs: pollIntervalSchema,
    studentMaxFileBytes: z.number().int().positive(),
    fileMaxUploadBytes: z.number().int().positive(),
    filePreviewTimeoutMs: z.number().int().positive(),
    auditDiffTimeoutMs: z.number().int().positive().default(2000),
    fileAllowedContentTypes: z
      .array(contentTypeSchema)
      .nonempty()
      .refine((types) => new Set(types).size === types.length),
    fileContentTypeGroups: z
      .record(z.string().trim().min(1), z.array(contentTypeSchema).nonempty())
      .refine(
        (groups) => {
          const types = Object.values(groups).flat();
          return new Set(types).size === types.length;
        },
        { message: '同一文件类型不能重复配置显示分组' },
      ),
    projectMaxFileBytes: z.number().int().positive(),
    projectMaxApprovalTargets: z.number().int().positive().default(10),
    projectMaxApprovalSteps: z.number().int().positive().default(20),
    projectMaxDetails: z.number().int().positive(),
    projectScoreMaxRules: z.number().int().positive().default(30),
    projectScoreRuleMaxDepth: z.number().int().positive().default(8),
    projectScoreRuleMaxNodes: z.number().int().positive().default(100),
    projectScoreRuleMaxValues: z.number().int().positive().default(100),
    projectScoreFormulaMaxLength: z.number().int().positive().default(4000),
    projectScorePreviewDelayMs: z.number().int().positive().default(300),
    monthlyWorkMaxLinks: z.number().int().positive(),
    monthlyWorkMaxFileBytes: z.number().int().positive(),
    partyBranchMaxFileBytes: z.number().int().positive(),
    cpcPersonMaxFileBytes: z.number().int().positive(),
    partyBranchRuleMaxDepth: z.number().int().positive(),
    partyBranchRuleMaxNodes: z.number().int().positive(),
    partyBranchRuleMaxValues: z.number().int().positive(),
    timelineMaxNodes: z.number().int().positive(),
    timelineMaxFieldsPerNode: z.number().int().positive(),
    timelineMaxRulesPerNode: z.number().int().positive(),
    timelineMaxDefinitionBytes: z.number().int().positive(),
    timelineMaxOffset: z.number().int().positive(),
    tablePageSize: z.number().int().positive(),
    tablePageSizeOptions: z.array(z.number().int().positive()).nonempty(),
  })
  .refine(
    (config) => {
      const types = Object.values(config.fileContentTypeGroups).flat();
      return config.fileAllowedContentTypes.every((type) => types.includes(type));
    },
    {
      message: '允许上传的文件类型必须配置显示分组',
      path: ['fileContentTypeGroups'],
    },
  )
  .refine(
    (config) =>
      config.tablePageSizeOptions.includes(config.tablePageSize) &&
      new Set(config.tablePageSizeOptions).size === config.tablePageSizeOptions.length,
    {
      message: '默认每页条数必须在不重复的分页选项中',
    },
  );

export type AppConfig = z.infer<typeof configSchema>;

declare global {
  interface Window {
    AGILE_WORKSPACE_CONFIG?: unknown;
  }
}

export function getConfig(): AppConfig {
  // 页面先加载 public/config.js，所有运行参数都从其读取。
  const result = configSchema.safeParse(window.AGILE_WORKSPACE_CONFIG);
  if (!result.success) {
    throw new Error('页面暂时无法加载，请联系管理员');
  }
  return result.data;
}
