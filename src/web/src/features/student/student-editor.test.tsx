import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { StudentEditor } from '@/features/student/student-editor';

afterEach(() => vi.unstubAllGlobals());

function renderEditor(canSensitive = true) {
  const onClose = vi.fn();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <StudentEditor
        workspaceID="w7m2k9p4s6n8c3x5"
        student={null}
        canQuery={false}
        canBasic
        canDetail={false}
        canSensitive={canSensitive}
        onClose={onClose}
      />
    </QueryClientProvider>,
  );
  return { editor: within(screen.getByRole('dialog', { name: '添加学生' })), onClose };
}

async function choose(label: string, option: string, create = false) {
  await userEvent.click(screen.getByRole('combobox', { name: label }));
  if (create) {
    fireEvent.change(screen.getByRole('searchbox', { name: `搜索${label}` }), {
      target: { value: option },
    });
  }
  await userEvent.click(screen.getByRole('option', { name: create ? `新建“${option}”` : option }));
}

async function fillAcademic() {
  fireEvent.change(screen.getByLabelText('学号 *'), { target: { value: '00000002' } });
  fireEvent.change(screen.getByLabelText('姓名 *'), { target: { value: '虚构新增学生' } });
  await choose('年级', '2026');
  await choose('状态', '在籍');
  await choose('学历', '学士');
  await choose('专业', '虚构专业', true);
  await choose('班级', '虚构班级', true);
  await choose('录取类型', '非定向');
}

describe('添加学生分步填写', () => {
  it('自由前进及跳步保留草稿，保存才检查必填项，步骤支持键盘切换', async () => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    const { editor } = renderEditor();
    expect(editor.getAllByRole('tab').map((tab) => tab.textContent.replace(/^\d/u, ''))).toEqual([
      '学籍资料',
      '个人经历',
      '联系家庭',
      '住宿信息',
      '敏感信息',
    ]);
    expect(editor.queryByRole('button', { name: '保存学生' })).not.toBeInTheDocument();
    await userEvent.click(editor.getByRole('button', { name: '下一步' }));
    expect(editor.getByRole('tab', { name: '个人经历' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByText('学号不能为空')).not.toBeInTheDocument();
    await choose('性别', '女');
    await choose('民族', '汉族');
    await userEvent.click(editor.getByRole('tab', { name: '联系家庭' }));
    for (const name of ['家长 1', '家长 2'])
      expect(editor.getByRole('region', { name })).toBeVisible();
    fireEvent.change(editor.getByLabelText('电话号码'), { target: { value: '虚构联系电话' } });
    await userEvent.click(editor.getByRole('tab', { name: '敏感信息' }));
    expect(editor.getByRole('tab', { name: '敏感信息' })).toHaveAttribute('aria-selected', 'true');
    await userEvent.click(editor.getByRole('button', { name: '保存学生' }));
    expect(await screen.findByText('学号不能为空')).toBeVisible();
    expect(editor.getByLabelText('学号 *')).toHaveFocus();
    await fillAcademic();
    const academic = editor.getByRole('tab', { name: '学籍资料' });
    academic.focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(editor.getByRole('tab', { name: '个人经历' })).toHaveFocus();
    expect(editor.getByRole('combobox', { name: '性别' })).toHaveTextContent('女');
    await userEvent.click(editor.getByRole('tab', { name: '联系家庭' }));
    expect(editor.getByLabelText('电话号码')).toHaveValue('虚构联系电话');
    expect(fetch).not.toHaveBeenCalled();
  });

  it('没有敏感权限时隐藏对应步骤，并按可见步骤调整总数和最后一步操作', async () => {
    const { editor } = renderEditor(false);
    expect(editor.getAllByRole('tab')).toHaveLength(4);
    expect(editor.queryByRole('tab', { name: '敏感信息' })).not.toBeInTheDocument();
    expect(editor.queryByLabelText('身份证号码 *')).not.toBeInTheDocument();
    expect(editor.getByText('第 1 / 4 步')).toBeVisible();
    await userEvent.click(editor.getByRole('tab', { name: '住宿信息' }));
    expect(editor.getByText('第 4 / 4 步')).toBeVisible();
    expect(editor.getByRole('button', { name: '保存学生' })).toBeVisible();
    expect(editor.queryByRole('button', { name: '下一步' })).not.toBeInTheDocument();
  });

  it('最后一步统一保存全部草稿，缺少身份证时定位，提交期间保护关闭', async () => {
    let resolveSave: (response: Response) => void = () => {
      throw new Error('保存请求尚未发起');
    };
    const fetch = vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          resolveSave = resolve;
        }),
    );
    vi.stubGlobal('fetch', fetch);
    const { editor, onClose } = renderEditor();
    await fillAcademic();
    await userEvent.click(editor.getByRole('button', { name: '下一步' }));
    await choose('性别', '女');
    await choose('民族', '汉族');
    await userEvent.click(editor.getByRole('button', { name: '下一步' }));
    fireEvent.change(editor.getByLabelText('电话号码'), { target: { value: '虚构联系电话' } });
    await userEvent.click(editor.getByRole('button', { name: '下一步' }));
    await choose('住宿类型', '校内');
    await userEvent.click(editor.getByRole('button', { name: '下一步' }));
    expect(fetch).not.toHaveBeenCalled();
    await userEvent.click(editor.getByRole('button', { name: '保存学生' }));
    expect(await screen.findByText('身份证号码不能为空')).toBeVisible();
    expect(editor.getByLabelText('身份证号码 *')).toHaveFocus();
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.change(editor.getByLabelText('身份证号码 *'), {
      target: { value: '00000020000102001X' },
    });
    await userEvent.click(editor.getByRole('button', { name: '保存学生' }));
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledOnce();
    });
    expect(editor.getByRole('button', { name: '关闭抽屉' })).toBeDisabled();
    expect(editor.getByRole('button', { name: '上一步' })).toBeDisabled();
    expect(editor.getAllByRole('tab').every((tab) => tab.hasAttribute('disabled'))).toBe(true);
    const [url, options] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('/v1/workspaces/w7m2k9p4s6n8c3x5/students');
    expect(options.method).toBe('POST');
    const body: unknown = JSON.parse(typeof options.body === 'string' ? options.body : '{}');
    expect(body).toMatchObject({
      student_no: '00000002',
      name: '虚构新增学生',
      phone: '虚构联系电话',
      gender: '女',
      accommodation_type: 'on_campus',
    });
    expect(body).toHaveProperty(
      'sensitive',
      expect.arrayContaining([{ field: 'identity_number', value: '00000020000102001X' }]),
    );
    resolveSave(
      new Response(
        JSON.stringify({
          trace: '4bf92f3577b34da6a3ce929d0e0e4736',
          error_msg: '',
          data: { id: 2, revision: 1 },
        }),
      ),
    );
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
    });
  });
});
