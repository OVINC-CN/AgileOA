import { z } from 'zod';

export const extraAssetsDirectory = 'extra-assets';

// 浏览器与生产构建共用部署地址和协议联系邮箱的校验规则。
export const deploymentFields = {
  contactEmail: z.email(),
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
