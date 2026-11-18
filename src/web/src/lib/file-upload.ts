import { getConfig } from '@/lib/config';

export const uploadStateLabels = {
  pending: '上传未完成',
  ready: '可用',
  deleting: '删除未完成',
} as const;

const extensionTypes: Record<string, string> = {
  pdf: 'application/pdf',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  apng: 'image/apng',
  gif: 'image/gif',
  webp: 'image/webp',
  bmp: 'image/bmp',
  tif: 'image/tiff',
  tiff: 'image/tiff',
  avif: 'image/avif',
  heic: 'image/heic',
  heif: 'image/heif',
  svg: 'image/svg+xml',
  ico: 'image/vnd.microsoft.icon',
  jxl: 'image/jxl',
  jp2: 'image/jp2',
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  ppt: 'application/vnd.ms-powerpoint',
  pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  txt: 'text/plain',
  json: 'application/json',
};

export function uploadAccept() {
  const types = getConfig().fileAllowedContentTypes;
  return [...types, ...(types.includes('application/pdf') ? ['.pdf'] : [])].join(',');
}

export function validateFileSize(file: File, maxBytes = getConfig().fileMaxUploadBytes) {
  if (file.size < 1) throw new Error('文件不能为空');
  const limit = Math.min(maxBytes, getConfig().fileMaxUploadBytes);
  if (file.size > limit) {
    throw new Error(`单个文件不能超过 ${String(limit / (1024 * 1024))} MB`);
  }
}

export function validateUploadFile(file: File) {
  validateFileSize(file);
  const contentType = fileContentType(file);
  if (
    !getConfig().fileAllowedContentTypes.some(
      (allowed) =>
        allowed === contentType ||
        (allowed.endsWith('/*') && contentType.startsWith(allowed.slice(0, -1))),
    )
  ) {
    throw new Error('文件类型不在允许上传的范围内');
  }
}

// 声明类型由 STS 策略约束；浏览器缺少类型时采用固定扩展名目录。
export function fileContentType(file: File) {
  const type = file.type.toLowerCase();
  return type && type !== 'application/octet-stream'
    ? type
    : (extensionTypes[file.name.split('.').at(-1)?.toLowerCase() ?? ''] ?? '');
}

export function uploadHint() {
  const config = getConfig();
  const labels = Object.entries(config.fileContentTypeGroups)
    .filter(([, types]) => types.some((type) => config.fileAllowedContentTypes.includes(type)))
    .map(([label]) => label);
  return `支持 ${labels.join('、')}，单个文件不超过 ${String(config.fileMaxUploadBytes / (1024 * 1024))} MB`;
}
