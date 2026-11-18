import { z } from 'zod';

import { apiGet } from '@/lib/api';
import { getConfig } from '@/lib/config';

const downloadLinkSchema = z.object({
  filename: z.string().min(1),
  url: z.url().refine((value) => new URL(value).protocol === 'https:'),
});

const officeContentTypes = {
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  ppt: 'application/vnd.ms-powerpoint',
  pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
} as const;

export type OfficePreviewKind = 'docx' | 'xlsx' | 'pptx';

export function officePreviewKind(contentType: string): OfficePreviewKind | null {
  switch (contentType) {
    case officeContentTypes.doc:
    case officeContentTypes.docx:
      return 'docx';
    case officeContentTypes.xls:
    case officeContentTypes.xlsx:
      return 'xlsx';
    case officeContentTypes.ppt:
    case officeContentTypes.pptx:
      return 'pptx';
    default:
      return null;
  }
}

export async function readPreviewFile(downloadURL: string, signal: AbortSignal): Promise<File> {
  // 沿用下载接口的鉴权与审计，读取 JSON 文件信息并禁止自动跟随跳转。
  const config = getConfig();
  const base = config.apiBaseURL.replace(/\/$/u, '');
  const path = new URL(downloadURL.slice(base.length), window.location.origin);
  const link = await apiGet(`${path.pathname}${path.search}`, downloadLinkSchema, signal, {
    redirect: 'error',
  });

  // 浏览器按 CDN 响应的缓存指令读取正文，供预览和下载复用。
  signal.throwIfAborted();
  const response = await fetch(link.url, {
    credentials: 'omit',
    redirect: 'error',
    cache: 'default',
    referrerPolicy: 'no-referrer',
    signal: AbortSignal.any([signal, AbortSignal.timeout(config.apiTimeoutMs)]),
  });
  if (!response.ok) throw new Error('文件读取失败，请重试');
  const blob = await response.blob();
  return new File([blob], link.filename, { type: blob.type });
}

export async function previewContentType(blob: Blob, filename: string): Promise<string | null> {
  const header = new Uint8Array(await blob.slice(0, 512).arrayBuffer());
  const text = new TextDecoder().decode(header);
  const tag = (start: number, end: number) => String.fromCharCode(...header.slice(start, end));
  // 存量附件可能保存为 application/octet-stream，优先按浏览器缓存中的文件头识别。
  if (text.startsWith('%PDF-')) return 'application/pdf';
  if (header[0] === 0x89 && tag(1, 4) === 'PNG') return 'image/png';
  if (header[0] === 0xff && header[1] === 0xd8 && header[2] === 0xff) return 'image/jpeg';
  if (/^GIF8[79]a/u.test(tag(0, 6))) return 'image/gif';
  if (tag(0, 4) === 'RIFF' && tag(8, 12) === 'WEBP') return 'image/webp';
  if (tag(0, 2) === 'BM') return 'image/bmp';
  if (tag(4, 8) === 'ftyp' && /^(avif|avis)$/u.test(tag(8, 12))) return 'image/avif';
  if (tag(0, 4) === 'II*\0' || tag(0, 4) === 'MM\0*') return 'image/tiff';
  if (tag(0, 4) === '\0\0\x01\0') return 'image/vnd.microsoft.icon';
  if (/^\s*(?:<\?xml[^>]*>\s*)?<svg[\s>]/iu.test(text)) return 'image/svg+xml';

  // Office 新版为 ZIP，旧版为复合文档；通用 MIME 的存量附件结合扩展名分流。
  const isZip = tag(0, 4) === 'PK\x03\x04';
  const isCompound = tag(0, 8) === '\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1';
  const extension = filename.split('.').at(-1)?.toLowerCase();
  if (isZip || isCompound) {
    const officeExtension = extension === 'docs' ? 'docx' : extension;
    if (officeExtension && Object.hasOwn(officeContentTypes, officeExtension)) {
      return officeContentTypes[officeExtension as keyof typeof officeContentTypes];
    }
    if (officePreviewKind(blob.type)) return blob.type;
  }

  // 其他图片格式交给浏览器图片解码器，无法显示时由预览窗口提示。
  if (blob.type.startsWith('image/')) return blob.type;
  if (extension && ['tif', 'tiff', 'heic', 'heif', 'ico', 'jxl', 'jp2'].includes(extension)) {
    return `image/${extension === 'tif' ? 'tiff' : extension}`;
  }
  return null;
}
