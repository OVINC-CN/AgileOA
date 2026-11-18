import { z } from 'zod';

import { apiPost } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { fileContentType, validateUploadFile } from '@/lib/file-upload';

const uploadSchema = z.object({
  upload_id: z.number().int().positive(),
  revision: z.number().int().positive(),
  upload: z.object({
    bucket: z.string().regex(/^[a-z0-9][a-z0-9-]{0,49}-\d+$/u),
    region: z.string().regex(/^[a-z][a-z0-9-]+$/u),
    key: z.string().min(1),
    content_type: z.string().min(1),
    tmp_secret_id: z.string().min(1),
    tmp_secret_key: z.string().min(1),
    security_token: z.string().min(1),
    start_time: z.number().int().positive(),
    expired_time: z.number().int().positive(),
  }),
});

// 凭据只保存在本次函数调用中；申请与确认沿用 Cookie，文件正文由 COS SDK 直传。
export async function uploadFile<T>(
  path: string,
  schema: z.ZodType<T>,
  file: File,
  fields: Record<string, unknown> = {},
  inline = false,
): Promise<T> {
  validateUploadFile(file);
  const prepared = await apiPost(path, uploadSchema, {
    ...fields,
    filename: file.name,
    size: file.size,
    content_type: fileContentType(file),
  });

  // 简单上传确保 STS 的 Content-Length 约束覆盖整个文件，不启用分片或断点续传。
  const { default: COS } = await import('cos-js-sdk-v5');
  const upload = prepared.upload;
  const client = new COS({
    SecretId: upload.tmp_secret_id,
    SecretKey: upload.tmp_secret_key,
    SecurityToken: upload.security_token,
    StartTime: upload.start_time,
    ExpiredTime: upload.expired_time,
    Protocol: 'https:',
    Timeout: getConfig().apiTimeoutMs,
  });
  try {
    await client.putObject({
      Bucket: upload.bucket,
      Region: upload.region,
      Key: upload.key,
      Body: file,
      ContentType: upload.content_type,
      ContentDisposition: inline ? 'inline' : 'attachment',
    });
  } catch {
    // SDK 原始错误可能含签名或对象地址，仅向用户提供固定提示。
    throw new Error('文件上传失败，请重试');
  }

  return apiPost(`${path}/complete`, schema, {
    upload_id: prepared.upload_id,
    revision: prepared.revision,
  });
}
