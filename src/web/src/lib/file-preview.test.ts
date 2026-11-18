import { Blob as NodeBlob } from 'node:buffer';

import { afterEach, expect, it, vi } from 'vitest';

import { ApiError } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { officePreviewKind, previewContentType, readPreviewFile } from '@/lib/file-preview';

const originalConfig = window.AGILE_WORKSPACE_CONFIG;
const cdnURL = 'https://cdn.example.com/fixture.pdf?eo-sign=fixture';
const link = { filename: '原始材料.pdf', url: cdnURL };
function response(data: unknown, status = 200) {
  return Response.json(
    { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data },
    { status },
  );
}

afterEach(() => {
  window.AGILE_WORKSPACE_CONFIG = originalConfig;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

it('先获取已审计的 JSON 下载地址，再按 CDN 响应缓存策略读取文件', async () => {
  const file = new Blob(['%PDF-1.7\nfixture']);
  const content = new Response();
  vi.spyOn(content, 'blob').mockResolvedValue(file);
  const fetch = vi
    .fn<typeof globalThis.fetch>()
    .mockResolvedValueOnce(response(link))
    .mockResolvedValueOnce(content);
  vi.stubGlobal('fetch', fetch);
  const path = '/v1/workspaces/fixture/attachments/7/download';
  const signal = new AbortController().signal;
  const cached = await readPreviewFile(path, signal);
  expect(cached).toBeInstanceOf(File);
  expect(cached.name).toBe('原始材料.pdf');
  expect(cached.size).toBe(file.size);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch).toHaveBeenNthCalledWith(
    1,
    path,
    expect.objectContaining({
      credentials: 'include',
      redirect: 'error',
    }),
  );
  expect(fetch).toHaveBeenNthCalledWith(
    2,
    cdnURL,
    expect.objectContaining({
      credentials: 'omit',
      redirect: 'error',
      cache: 'default',
      referrerPolicy: 'no-referrer',
    }),
  );
  expect(new Headers(fetch.mock.calls[1]?.[1]?.headers).has('Authorization')).toBe(false);
});

it('保留 API 地址前缀及已有筛选参数，不增加下载模式参数', async () => {
  window.AGILE_WORKSPACE_CONFIG = {
    ...getConfig(),
    apiBaseURL: 'https://api.example.com/service/',
  };
  const fetch = vi
    .fn<typeof globalThis.fetch>()
    .mockResolvedValueOnce(response(link))
    .mockResolvedValueOnce(new Response('fixture'));
  vi.stubGlobal('fetch', fetch);
  await readPreviewFile(
    'https://api.example.com/service/v1/workspaces/fixture/attachments/7/download?scope=mine',
    new AbortController().signal,
  );
  expect(fetch.mock.calls[0]?.[0]).toBe(
    'https://api.example.com/service/v1/workspaces/fixture/attachments/7/download?scope=mine',
  );
});

it('接口拒绝、无效下载地址或请求取消时，不继续请求 CDN', async () => {
  const fetch = vi.fn<typeof globalThis.fetch>();
  vi.stubGlobal('fetch', fetch);
  fetch.mockResolvedValueOnce(response(null, 403));
  await expect(readPreviewFile('/download', new AbortController().signal)).rejects.toBeInstanceOf(
    ApiError,
  );
  expect(fetch).toHaveBeenCalledOnce();

  fetch.mockClear();
  fetch.mockResolvedValueOnce(response({ ...link, url: 'javascript:invalid' }));
  await expect(readPreviewFile('/download', new AbortController().signal)).rejects.toBeInstanceOf(
    ApiError,
  );
  expect(fetch).toHaveBeenCalledOnce();

  fetch.mockClear();
  const controller = new AbortController();
  fetch.mockImplementationOnce(() => {
    controller.abort();
    return Promise.resolve(response(link));
  });
  await expect(readPreviewFile('/download', controller.signal)).rejects.toBeDefined();
  expect(fetch).toHaveBeenCalledOnce();
});

it('对下载到浏览器的内容识别 PDF、图片，兼容旧附件的通用 MIME', async () => {
  const blob = (bytes: string | Uint8Array) =>
    new NodeBlob([bytes], { type: 'application/octet-stream' }) as Blob;
  await expect(previewContentType(blob('%PDF-1.7\nfixture'), '附件 #7')).resolves.toBe(
    'application/pdf',
  );
  await expect(
    previewContentType(blob(new Uint8Array([0x89, 0x50, 0x4e, 0x47, 13, 10, 26, 10])), '附件 #8'),
  ).resolves.toBe('image/png');
  const webp = new Uint8Array([82, 73, 70, 70, 0x80, 0, 0, 0, 87, 69, 66, 80]);
  await expect(previewContentType(blob(webp), '附件 #9')).resolves.toBe('image/webp');
  await expect(previewContentType(blob('普通文本'), '其他.pdf')).resolves.toBeNull();
});

it.each([
  ['doc', 'application/msword', 'docx', false],
  ['docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'docx', true],
  ['docs', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'docx', true],
  ['xls', 'application/vnd.ms-excel', 'xlsx', false],
  ['xlsx', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'xlsx', true],
  ['ppt', 'application/vnd.ms-powerpoint', 'pptx', false],
  [
    'pptx',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    'pptx',
    true,
  ],
] as const)('识别 %s 附件，兼容大小写扩展名和通用 MIME', async (extension, type, kind, zip) => {
  const header = zip
    ? new Uint8Array([0x50, 0x4b, 0x03, 0x04])
    : new Uint8Array([0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1]);
  const blob = new NodeBlob([header], { type: 'application/octet-stream' }) as Blob;
  const detected = await previewContentType(blob, `材料.${extension.toUpperCase()}`);
  expect(detected).toBe(type);
  expect(officePreviewKind(detected ?? '')).toBe(kind);
});

it('Office 识别须匹配容器文件头，不将普通压缩包或伪装文本当作文档', async () => {
  const zipHeader = new Uint8Array([0x50, 0x4b, 0x03, 0x04]);
  const zip = new NodeBlob([zipHeader]) as Blob;
  await expect(previewContentType(zip, '普通压缩包.zip')).resolves.toBeNull();
  const text = new NodeBlob(['普通文本'], { type: 'application/msword' }) as Blob;
  await expect(previewContentType(text, '材料.doc')).resolves.toBeNull();
  const known = new NodeBlob([zipHeader], { type: 'application/vnd.ms-excel' }) as Blob;
  await expect(previewContentType(known, '附件 #7')).resolves.toBe('application/vnd.ms-excel');
  const pdf = new NodeBlob(['%PDF-1.7\nfixture']) as Blob;
  await expect(previewContentType(pdf, '材料.docx')).resolves.toBe('application/pdf');
});
