import { beforeEach, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { uploadFile } from '@/lib/cos-upload';
import { cosMock, preparedUpload } from '@/test/mock-cos';

const trace = '0123456789abcdef0123456789abcdef';
const respond = (data: unknown) =>
  new Response(JSON.stringify({ trace, error_msg: '', data }), {
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  cosMock.putObject.mockReset().mockResolvedValue({ statusCode: 200 });
});

it('正文只发给 COS，申请与确认只发送元数据和服务端上传编号', async () => {
  const prepared = preparedUpload();
  const request = vi
    .fn<(path: string, options: RequestInit) => Promise<Response>>()
    .mockResolvedValueOnce(respond(prepared))
    .mockResolvedValueOnce(respond({ id: 1 }));
  vi.stubGlobal('fetch', request);
  const file = new File(['fixture'], '材料.pdf', { type: 'application/pdf' });
  await expect(
    uploadFile('/v1/fixture/attachments', z.object({ id: z.number() }), file),
  ).resolves.toEqual({ id: 1 });
  expect(
    JSON.parse(
      typeof request.mock.calls[0]?.[1].body === 'string' ? request.mock.calls[0][1].body : '',
    ),
  ).toEqual({
    filename: file.name,
    size: file.size,
    content_type: 'application/pdf',
  });
  expect(cosMock.putObject).toHaveBeenCalledWith({
    Bucket: prepared.upload.bucket,
    Region: prepared.upload.region,
    Key: prepared.upload.key,
    Body: file,
    ContentType: 'application/pdf',
    ContentDisposition: 'attachment',
  });
  expect(request.mock.calls[1]?.[0]).toBe('/v1/fixture/attachments/complete');
  expect(
    JSON.parse(
      typeof request.mock.calls[1]?.[1].body === 'string' ? request.mock.calls[1][1].body : '',
    ),
  ).toEqual({
    upload_id: 1,
    revision: 1,
  });
  for (const [, options] of request.mock.calls) {
    expect(options.body).not.toContain('fixture-key');
    expect(new Headers(options.headers).get('X-CSRF-Protection')).toBe('1');
  }
});

it('COS 上传失败保留待完成记录，不请求完成接口，也不展示原始 SDK 错误', async () => {
  const request = vi
    .fn<(path: string, options: RequestInit) => Promise<Response>>()
    .mockResolvedValue(respond(preparedUpload()));
  vi.stubGlobal('fetch', request);
  cosMock.putObject.mockRejectedValueOnce(new Error('fixture-secret-and-signed-url'));
  await expect(
    uploadFile(
      '/v1/fixture/attachments',
      z.null(),
      new File(['fixture'], '材料.pdf', { type: 'application/pdf' }),
    ),
  ).rejects.toThrow('文件上传失败，请重试');
  expect(request).toHaveBeenCalledOnce();
});
