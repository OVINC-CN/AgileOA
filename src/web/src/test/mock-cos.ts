import { vi } from 'vitest';

const cosMock = vi.hoisted(() => ({
  putObject: vi.fn().mockResolvedValue({ statusCode: 200 }),
}));

vi.mock('cos-js-sdk-v5', () => ({
  default: class {
    putObject = cosMock.putObject;
  },
}));

export { cosMock };

export function preparedUpload(contentType = 'application/pdf', revision = 1) {
  return {
    upload_id: 1,
    revision,
    upload: {
      bucket: 'fixture-upload-1250000000',
      region: 'ap-guangzhou',
      key: 'workspaces/fixture/012345abcd/材料.pdf',
      content_type: contentType,
      tmp_secret_id: 'fixture-id',
      tmp_secret_key: 'fixture-key',
      security_token: 'fixture-token',
      start_time: 1794873600,
      expired_time: 1794875400,
    },
  };
}
