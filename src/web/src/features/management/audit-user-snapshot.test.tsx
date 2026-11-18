import { render, screen, within } from '@testing-library/react';
import { expect, it } from 'vitest';
import { z } from 'zod';

import { parseAuditJSON } from '@/features/management/audit-json';
import { AuditUserSnapshot } from '@/features/management/audit-user-snapshot';

it('完整快照按身份、资料和状态分组，保留未知嵌套数据、长整数和空值语义', () => {
  const snapshot = z.record(z.string(), z.unknown()).parse(
    parseAuditJSON(`{
    "id": 18446744073709551615,
    "username": "fixture",
    "display_name": "虚构用户",
    "is_admin": false,
    "profile": {"quota": 0, "contact": null, "groups": ["fictional", false]}
  }`),
  );
  render(<AuditUserSnapshot snapshot={snapshot} />);
  const identity = within(screen.getByRole('region', { name: '身份标识' }));
  expect(identity.getByText('账号 ID')).toBeVisible();
  expect(identity.getByText('18446744073709551615')).toBeVisible();
  const account = within(screen.getByRole('region', { name: '账号资料' }));
  expect(account.getByText('用户名')).toBeVisible();
  expect(account.getByText('fixture')).toBeVisible();
  expect(account.getByText('虚构用户')).toBeVisible();
  expect(within(screen.getByRole('region', { name: '权限状态' })).getByText('false')).toBeVisible();
  const extra = within(screen.getByRole('region', { name: '其他资料' }));
  expect(extra.getByText('profile')).toBeVisible();
  expect(extra.getByText('0')).toBeVisible();
  expect(extra.getByText('-')).toBeVisible();
  expect(extra.getByText('fictional')).toBeVisible();
  expect(extra.getByText('false')).toBeVisible();
  expect(extra.queryByText('[object Object]')).not.toBeInTheDocument();
});
