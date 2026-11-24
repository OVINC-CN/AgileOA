import { z } from 'zod';

export const extraAssetsDirectory = 'extra-assets';
export const fileViewerAssetsDirectory = `${extraAssetsDirectory}/file-viewer`;

// 浏览器与生产构建共用部署字段的校验规则。
export const deploymentFields = {
  contactEmail: z.email(),
  fileViewerAssetsPath: z
    .string()
    .regex(/^extra-assets\/file-viewer(?:\/[a-f0-9]{12})?$/u)
    .default(fileViewerAssetsDirectory),
  apiBaseURL: z.string().refine((value) => {
    if (value === '') {
      return true;
    }
    try {
      const url = new URL(value);
      return url.protocol === 'http:' || url.protocol === 'https:';
    } catch {
      return false;
    }
  }),
};
