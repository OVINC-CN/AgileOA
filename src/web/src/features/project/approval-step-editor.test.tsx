import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';

import { type ApprovalStep, approvalStepSchema } from '@/features/project/api';
import { ApprovalStepEditor } from '@/features/project/approval-step-editor';

afterEach(() => {
  vi.unstubAllGlobals();
});

function Editor() {
  const [client] = useState(() => new QueryClient());
  const [steps, setSteps] = useState<ApprovalStep[]>([
    approvalStepSchema.parse({ kind: 'applicant', reviewer_label: '申报人' }),
    approvalStepSchema.parse({ kind: 'applicant', reviewer_label: '申报人' }),
  ]);
  return (
    <QueryClientProvider client={client}>
      <ApprovalStepEditor workspaceID="w7m2k9p4s6n8c3x5" steps={steps} onChange={setSteps} />
    </QueryClientProvider>
  );
}

it('各级默认允许编辑，开关独立且随级别移动，切换处理人类型后保留', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(Response.json({ trace: '', error_msg: '', data: { items: [] } }))),
  );
  render(<Editor />);
  const user = userEvent.setup();
  const checkbox = (position: number) =>
    screen.getByRole('checkbox', { name: `第 ${String(position)} 级允许编辑申报内容` });
  expect(checkbox(1)).toBeChecked();
  expect(checkbox(2)).toBeChecked();
  await user.click(checkbox(1));
  expect(checkbox(1)).not.toBeChecked();
  expect(checkbox(2)).toBeChecked();
  await user.click(screen.getByRole('button', { name: '下移第 1 级' }));
  expect(checkbox(1)).toBeChecked();
  expect(checkbox(2)).not.toBeChecked();
  await user.click(screen.getByRole('combobox', { name: '第 2 级处理人类型' }));
  await user.click(screen.getByRole('option', { name: '班委' }));
  expect(checkbox(2)).not.toBeChecked();
  await user.click(screen.getByRole('button', { name: '删除第 1 级' }));
  expect(checkbox(1)).not.toBeChecked();
  await user.click(screen.getByRole('button', { name: '添加审批级别' }));
  expect(checkbox(1)).not.toBeChecked();
  expect(checkbox(2)).toBeChecked();
});
