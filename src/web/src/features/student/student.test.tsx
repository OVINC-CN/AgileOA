import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, MemoryRouter, RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppLayout } from '@/components/app-layout';
import { emptyStudent } from '@/features/student/api';
import { birthDateFromIdentityNumber, sensitiveAction } from '@/features/student/fields';
import { StudentAvatar } from '@/features/student/student-avatar';
import { StudentEditor } from '@/features/student/student-editor';
import { StudentPage } from '@/features/student/student-page';
import { currentUserKey } from '@/features/user/api';
import { xlsxContentType } from '@/lib/spreadsheet';
import { OverviewPage } from '@/pages/overview-page';
import { WorkspacePage } from '@/pages/workspace-page';
import { WorkspaceStudentsPage } from '@/pages/workspace-students-page';
import { preparedUpload } from '@/test/mock-cos';

const first = 'w7m2k9p4s6n8c3x5';
const second = 'r4v8b2m6k9d3p5s7';
const fixtureIdentityNumber = '00000020000102001X';
const student = {
  ...emptyStudent,
  id: 1,
  student_no: '00000001',
  name: '虚构学生甲',
  birth_date: '2000-01-02',
  grade: '2026级',
  status: '自定义状态',
  education_level: '博士',
  gender: '女',
  ethnicity: '汉族',
  accommodation_type: 'on_campus',
  political_status: '群众',
  campus: '虚构园区',
  building: '1号楼',
  room: '101',
  phone: 'FICTIONAL-PHONE-001',
  major: '虚构专业',
  class_name: '2026级虚构班',
  class_position: '班长',
  admission_type: '非定向',
  arrival_month: '2026-09',
  previous_school_or_employer: '虚构大学',
  previous_graduation_month: '2026-06',
  advisor: '虚构导师',
  advisor_no: '00001',
  advisor_phone: '虚构导师电话',
  origin: '虚构户籍市',
  native_place: '虚构籍贯县',
  revision: 1,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const sensitiveValues = [
  { field: 'identity_number', value: fixtureIdentityNumber },
  { field: 'home_address', value: '虚构地址1号' },
  { field: 'only_child', value: 'yes' },
  { field: 'single_parent', value: 'no' },
  { field: 'orphan', value: 'unknown' },
  { field: 'student_loan', value: 'no' },
  { field: 'registered_poverty', value: 'unknown' },
  { field: 'has_religion', value: 'yes' },
  { field: 'religion_category', value: '天主教' },
  { field: 'religion_identification', value: '疑似' },
  { field: 'religion_start_year', value: '2020年' },
  { field: 'family_religion', value: '虚构家庭成员情况' },
  { field: 'medical_history', value: '虚构健康备注' },
  { field: 'special_circumstances', value: '虚构专项备注' },
];
const workspace = {
  id: first,
  name: '虚构空间甲',
  description: '',
  is_enabled: true,
  revision: 1,
  created_at: student.created_at,
  updated_at: student.updated_at,
};
const counts = { total: 12, class_count: 3, religious_count: 2, off_campus_count: 4 };
const avatarURL = 'https://cdn.example.com/avatar.png?eo-sign=fixture';
const updatedAvatarURL = 'https://cdn.example.com/new-avatar.png?eo-sign=fixture';
const listing = () => ({ items: [student], total: 1, page: 1, page_size: 5 });
const queryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  });
function respond(data: unknown) {
  return new Response(
    JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }),
  );
}
function mockAPI(
  actions: string[],
  accommodationType = student.accommodation_type,
  includeOffCampusDetails = true,
) {
  const calls: { url: string; options: RequestInit }[] = [];
  let currentAvatarURL = avatarURL;
  let revision = student.revision;
  let offCampus = {
    reason: '虚构原因',
    start_date: '2026-09-01',
    end_date: '',
    category: '',
    rental_method: '',
    landlord_name: '',
    landlord_phone: '',
    roommate_name: '',
    roommate_unit: '',
    roommate_phone: '',
    address: '虚构校外地址',
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, options: RequestInit) => {
      calls.push({ url, options });
      let data: unknown = {
        ...listing(),
        items: [{ ...student, revision, accommodation_type: accommodationType }],
      };
      if (url === `/v1/workspaces/${first}`) data = { workspace, actions };
      else if (url.includes('/statistics?')) data = counts;
      else if (url.includes('/export?'))
        data = {
          filename: url.includes('format=off_campus') ? '校外住宿明细表.xlsx' : 'XJ 明细表.xlsx',
          content_type: xlsxContentType,
          content: btoa('PK'),
        };
      else if (url.includes('-options?') || url.includes('/options?')) {
        const field = new URL(url, 'https://fixture.example').searchParams.get('field');
        const value =
          field === 'grade'
            ? '2026级'
            : field === 'status'
              ? student.status
              : field === 'education_level'
                ? '博士'
                : field === 'has_religion'
                  ? 'yes'
                  : field === 'class_position'
                    ? '班长'
                    : '2026级虚构班';
        data = [{ value, label: field === 'has_religion' ? '是' : value }];
      } else if (url.endsWith('/sensitive'))
        data =
          options.method === 'PUT'
            ? { id: 1, revision: 2 }
            : { student_id: 1, revision: 1, values: sensitiveValues };
      else if (url.endsWith('/1/avatar')) {
        data = preparedUpload('image/png');
      } else if (url.endsWith('/1/avatar/complete')) {
        currentAvatarURL = updatedAvatarURL;
        data = { id: 1, revision: 2 };
      } else if (url.endsWith('/1')) {
        if (options.method === 'PUT') {
          if (typeof options.body !== 'string') throw new Error('缺少学生编辑 JSON 请求');
          const body = JSON.parse(options.body) as { off_campus_details?: typeof offCampus };
          offCampus = body.off_campus_details ?? offCampus;
          revision += 1;
          data = { id: 1, revision };
        } else {
          data = {
            ...student,
            revision,
            accommodation_type: accommodationType,
            avatar_url: currentAvatarURL,
            off_campus_details:
              accommodationType === 'off_campus' && includeOffCampusDetails ? offCampus : undefined,
          };
        }
      }
      return Promise.resolve(respond(data));
    }),
  );
  return calls;
}
function renderWorkspace(query: Record<string, string> = {}, hash = '') {
  const search = new URLSearchParams({ workspace: first, ...query });
  const router = createMemoryRouter(
    [
      {
        path: '/stu',
        Component: WorkspacePage,
        children: [{ path: 'roster', Component: WorkspaceStudentsPage }],
      },
    ],
    { initialEntries: [`/stu/roster?${search.toString()}${hash}`] },
  );
  render(
    <QueryClientProvider client={queryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}
afterEach(() => {
  vi.unstubAllGlobals();
});

describe('学生出生日期与头像', () => {
  it.each([
    ['00000020040229001x', '2004-02-29'],
    [' 00000020000102001X ', '2000-01-02'],
    ['00000020010229001X', ''],
    ['00000020000431001X', ''],
    ['00000020001301001X', ''],
    ['00000020000102001', ''],
    ['', ''],
  ])('从身份证号码 %s 生成出生日期', (identity, expected) => {
    expect(birthDateFromIdentityNumber(identity)).toBe(expected);
  });

  it('头像加载结束后延迟退出，切换地址后重新加载，失败后显示占位', async () => {
    const view = render(<StudentAvatar name={student.name} src={avatarURL} />);
    expect(screen.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    const image = screen.getByRole('img', { name: `${student.name}的头像` });
    expect(image).toHaveClass('opacity-0');
    fireEvent.load(image);
    expect(screen.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    await waitFor(() => {
      expect(screen.queryByRole('status', { name: '正在加载头像' })).not.toBeInTheDocument();
    });
    expect(image).toHaveClass('opacity-100');
    expect(screen.getByRole('button', { name: '预览头像' })).toBeEnabled();

    view.rerender(<StudentAvatar name={student.name} src={updatedAvatarURL} />);
    expect(screen.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    fireEvent.error(screen.getByRole('img', { name: `${student.name}的头像` }));
    await waitFor(() => {
      expect(screen.queryByRole('status', { name: '正在加载头像' })).not.toBeInTheDocument();
    });
    expect(screen.getByLabelText(`${student.name}暂无头像`)).toBeVisible();

    view.rerender(<StudentAvatar name={student.name} src="" loading />);
    expect(screen.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    view.rerender(<StudentAvatar name={student.name} src="" />);
    await waitFor(() => {
      expect(screen.queryByRole('status', { name: '正在加载头像' })).not.toBeInTheDocument();
    });
  });
});

describe('工作空间学生信息', () => {
  it('首页展示当前日期与时间，工作空间独立显示且系统管理保留在用户菜单', async () => {
    const account = {
      id: 1,
      username: 'fixture.admin',
      display_name: '虚构管理员',
      email: '',
      is_admin: true,
      password_login_enabled: true,
      password_change_required: false,
    };
    const fetchMock = vi.fn<(url: string) => Promise<Response>>();
    fetchMock.mockImplementation((url) => {
      const address = new URL(url, 'https://fixture.example');
      if (address.pathname === '/v1/home/calendar') {
        return Promise.resolve(
          respond({ date: address.searchParams.get('date'), holidays: [], next: null }),
        );
      }
      return Promise.resolve(
        respond(url === '/v1/notifications/unread' ? { unread_count: 0 } : account),
      );
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = queryClient();
    client.setQueryData(currentUserKey, account);
    const user = userEvent.setup();
    const router = createMemoryRouter([
      { Component: AppLayout, children: [{ path: '/', Component: OverviewPage }] },
    ]);
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('group', { name: '当前日期' })).toBeVisible();
    expect(screen.getByRole('timer')).toBeVisible();
    expect(screen.queryByRole('heading', { name: '我的工作空间' })).not.toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([url]) => url.startsWith('/v1/workspaces'))).toBe(false);
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
    expect(screen.queryByText('我的账号')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '选择工作空间' })).toBeVisible();
    await user.click(screen.getByRole('button', { name: '当前用户 虚构管理员' }));
    expect(screen.queryByRole('menuitem', { name: '工作空间' })).not.toBeInTheDocument();
    expect(await screen.findByRole('menuitem', { name: '系统管理' })).toBeVisible();
  });

  it('任职状态和任职使用相同下拉，筛选枚举按权限获取并屏蔽密码自动填充', async () => {
    const calls = mockAPI(['student.list', 'student.export']);
    renderWorkspace();
    await screen.findByText(student.name);
    await userEvent.click(screen.getByRole('button', { name: '导出' }));
    expect(await screen.findByRole('menuitem', { name: '导出所有数据' })).toBeVisible();
    expect(screen.queryByRole('menuitem', { name: '导出 XJ 明细表' })).not.toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('combobox', { name: '班内任职状态' })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /更多筛选/u }));
    const position = screen.getByRole('combobox', { name: '班内任职' });
    expect(screen.getByRole('combobox', { name: '是否有宗教信仰' })).toBeDisabled();
    const status = screen.getByRole('combobox', { name: '班内任职状态' });
    expect(status.tagName).toBe('BUTTON');
    expect(position.tagName).toBe('BUTTON');
    expect(status.className).toBe(position.className);
    expect(screen.getByRole('searchbox', { name: '学号或姓名' })).toHaveAttribute(
      'data-1p-ignore',
      'true',
    );
    await userEvent.click(status);
    expect(screen.getByRole('searchbox', { name: '搜索班内任职状态' })).toHaveAttribute(
      'autocomplete',
      'off',
    );
    await userEvent.click(screen.getByRole('option', { name: '有任职' }));
    await userEvent.click(position);
    await userEvent.click(await screen.findByRole('option', { name: '班长' }));
    await userEvent.click(screen.getByRole('button', { name: '搜索' }));
    await waitFor(() => {
      expect(
        calls.some(
          ({ url }) => url.includes('position_status=assigned') && url.includes('class_position='),
        ),
      ).toBe(true);
    });
    expect(calls.every(({ url }) => !url.includes('/sensitive'))).toBe(true);
  });

  it.each([
    ['religion', 'XJ 明细表'],
    ['off_campus', '校外住宿明细表'],
  ])('%s 明细使用已提交筛选并下载指定 XLSX', async (format, title) => {
    const calls = mockAPI(['student.list', 'student.export', sensitiveAction]);
    vi.stubGlobal(
      'URL',
      class extends URL {
        static override createObjectURL = vi.fn(() => 'blob:fixture');
        static override revokeObjectURL = vi.fn();
      },
    );
    const download = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      expect(this.download).toBe(`${title}.xlsx`);
    });
    try {
      renderWorkspace();
      const user = userEvent.setup();
      const button = await screen.findByRole('button', { name: '导出' });
      const keyword = screen.getByRole('searchbox', { name: '学号或姓名' });
      await user.type(keyword, '已提交');
      await user.click(screen.getByRole('button', { name: '搜索' }));
      await user.type(keyword, '未提交');
      await user.click(button);
      await user.click(
        await screen.findByRole('menuitem', {
          name: `导出${format === 'religion' ? ' ' : ''}${title}`,
        }),
      );

      await waitFor(() => {
        expect(download).toHaveBeenCalledOnce();
      });
      const request = calls.find(({ url }) => url.includes('/export?'));
      const url = new URL(request?.url ?? '', 'https://fixture.example');
      expect(url.pathname).toBe(`/v1/workspaces/${first}/students/export`);
      expect(Object.fromEntries(url.searchParams)).toEqual({
        format,
        keyword: '已提交',
        status: '在籍',
      });
      await user.click(button);
      await user.click(await screen.findByRole('menuitem', { name: '导出所有数据' }));
      const drawer = within(await screen.findByRole('dialog', { name: '导出学生' }));
      expect(drawer.getByRole('checkbox', { name: '包含敏感信息' })).toBeVisible();
    } finally {
      download.mockRestore();
    }
  });

  it.each([false, true])('详情使用底部分区，敏感信息处理权限：%s', async (allowed) => {
    const calls = mockAPI([
      'student.list',
      'student.detail',
      ...(allowed ? [sensitiveAction] : []),
    ]);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('link', { name: '详情' }));
    const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
    const tabs = await drawer.findByRole('tablist', { name: '学生信息分区' });
    expect(tabs.closest('[data-slot="drawer-footer-navigation"]')).toBeInTheDocument();
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['学籍资料', '联系住宿', ...(allowed ? ['敏感信息'] : [])]);
    const academicTab = drawer.getByRole('tab', { name: '学籍资料' });
    expect(academicTab).toHaveAttribute('aria-selected', 'true');
    for (const name of ['学籍信息', '来校前经历', '导师信息', '个人信息'])
      expect(drawer.getByRole('heading', { name })).toBeVisible();
    expect(drawer.getByText(student.previous_school_or_employer)).toBeVisible();
    expect(drawer.getByText(student.arrival_month)).toBeVisible();
    expect(drawer.getByText(student.previous_graduation_month)).toBeVisible();
    const academic = drawer.getByRole('button', { name: '学籍信息' });
    await userEvent.click(academic);
    expect(academic).toHaveAttribute('aria-expanded', 'false');
    expect(drawer.getByText(student.advisor_no)).toBeVisible();
    expect(drawer.getByText('虚构户籍市')).toBeVisible();
    expect(drawer.getByText('虚构籍贯县')).toBeVisible();
    await userEvent.click(drawer.getByRole('tab', { name: '联系住宿' }));
    const contact = within(drawer.getByRole('tabpanel', { name: '联系住宿' }));
    for (const name of ['联系方式', '家庭联系', '住宿信息'])
      expect(contact.getByRole('heading', { name })).toBeVisible();
    expect(drawer.queryByRole('heading', { name: '学籍信息' })).not.toBeInTheDocument();
    expect(drawer.queryByText('宗教信仰起始年份')).not.toBeInTheDocument();
    expect(drawer.queryByRole('button', { name: '显示原始值' })).not.toBeInTheDocument();
    expect(calls.some(({ url }) => url.endsWith('/sensitive'))).toBe(false);
    if (!allowed) {
      expect(drawer.queryByRole('tab', { name: '敏感信息' })).not.toBeInTheDocument();
      return;
    }
    const sensitiveTab = drawer.getByRole('tab', { name: '敏感信息' });
    await userEvent.click(sensitiveTab);
    const sensitive = within(drawer.getByRole('tabpanel', { name: '敏感信息' }));
    expect(await sensitive.findByText(fixtureIdentityNumber)).toBeVisible();
    expect(sensitive.getByText('2020年')).toBeVisible();
    expect(sensitive.getByRole('button', { name: '复制身份证号码' })).toBeInTheDocument();
    expect(drawer.queryByRole('button', { name: /^(显示|隐藏)原始值$/u })).not.toBeInTheDocument();
    expect(calls.filter(({ url }) => url.endsWith('/sensitive'))).toHaveLength(1);
    await userEvent.click(academicTab);
    expect(drawer.queryByText(fixtureIdentityNumber)).not.toBeInTheDocument();
    await userEvent.click(sensitiveTab);
    expect(await sensitive.findByText(fixtureIdentityNumber)).toBeVisible();
    expect(calls.filter(({ url }) => url.endsWith('/sensitive'))).toHaveLength(2);
  });

  it('校外住宿并入普通编辑，详情字段可复制', async () => {
    const calls = mockAPI(['student.list', 'student.detail', 'student.update'], 'off_campus');
    renderWorkspace();
    const user = userEvent.setup();
    const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    await user.click(await screen.findByRole('link', { name: '详情' }));
    const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
    await user.click(await drawer.findByRole('tab', { name: '联系住宿' }));
    const address = drawer.getByText('校外居住地详细地址');
    const reason = drawer.getByText('校外住宿原因');
    expect(address.parentElement?.nextElementSibling).toBe(reason.parentElement);
    expect(reason.parentElement).toHaveClass('sm:col-span-2');
    await user.hover(drawer.getByText('虚构原因'));
    await user.click(drawer.getByRole('button', { name: '复制校外住宿原因' }));
    expect(copy).toHaveBeenCalledWith('虚构原因');
    copy.mockRestore();
    expect(drawer.queryByRole('button', { name: '校外住宿信息管理' })).not.toBeInTheDocument();
    expect(drawer.queryByRole('button', { name: '编辑校外住宿信息' })).not.toBeInTheDocument();

    await user.click(drawer.getByRole('button', { name: '编辑' }));
    const editor = within(await screen.findByRole('dialog', { name: '编辑学生' }));
    await user.click(editor.getByRole('tab', { name: '联系住宿' }));
    expect(editor.getByRole('combobox', { name: '校外住宿开始日期' })).toHaveTextContent(
      '2026年9月1日',
    );
    const addressInput = editor.getByRole('textbox', { name: '校外居住地详细地址' });
    expect(addressInput.tagName).toBe('TEXTAREA');
    await user.type(addressInput, '{Enter}虚构房间');
    await user.click(editor.getByRole('combobox', { name: '校外住宿结束日期' }));
    const datePicker = within(screen.getByRole('dialog', { name: '校外住宿结束日期选择' }));
    const dateInput = datePicker.getByRole('textbox', { name: '输入日期' });
    await user.clear(dateInput);
    await user.type(dateInput, '2027-06-30');
    await user.click(datePicker.getByRole('button', { name: '确定' }));
    await user.type(editor.getByRole('textbox', { name: '校外住宿类别' }), '虚构类别');
    await user.click(editor.getByRole('button', { name: '保存' }));
    expect(await drawer.findByText('2027-06-30')).toBeVisible();
    const save = calls.find(
      ({ url, options }) => url.endsWith('/students/1') && options.method === 'PUT',
    );
    expect(save?.options.body).toContain('"off_campus_details":');
    expect(save?.options.body).toContain('"end_date":"2027-06-30"');
    expect(save?.options.body).toContain('"address":"虚构校外地址\\n虚构房间"');
  });

  it.each(['off_campus', 'on_campus'])(
    '仅校外学生显示住宿明细，查看权限不授予编辑：%s',
    async (type) => {
      mockAPI(['student.list', 'student.detail'], type);
      renderWorkspace();
      await userEvent.click(await screen.findByRole('link', { name: '详情' }));
      const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
      await userEvent.click(await drawer.findByRole('tab', { name: '联系住宿' }));
      expect(drawer.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
      if (type === 'off_campus') {
        expect(await drawer.findByText('虚构原因')).toBeVisible();
      } else {
        expect(drawer.queryByText('校外住宿原因')).not.toBeInTheDocument();
      }
    },
  );

  it('校外住宿学生尚未填写明细时仍显示校外住宿信息', async () => {
    mockAPI(['student.list', 'student.detail'], 'off_campus', false);
    renderWorkspace({ student_id: '1' });
    const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
    await userEvent.click(await drawer.findByRole('tab', { name: '联系住宿' }));
    const section = within(drawer.getByRole('region', { name: '住宿信息' }));
    expect(section.getByText('校外住宿原因')).toBeVisible();
    expect(section.getByText('校外住宿开始日期').nextElementSibling).toHaveTextContent('-');
  });

  it('学生详情从列表打开抽屉，关闭后保留搜索和列表状态', async () => {
    mockAPI(['student.list', 'student.detail']);
    const router = renderWorkspace({ source: 'fixture' });
    const user = userEvent.setup();
    await screen.findByText(student.name);
    await user.type(screen.getByRole('searchbox', { name: '学号或姓名' }), '虚构');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    await user.click(screen.getByRole('link', { name: '详情' }));
    const drawer = await screen.findByRole('dialog', { name: '学生详情' });
    expect(await within(drawer).findByRole('tabpanel', { name: '学籍资料' })).toBeVisible();
    expect(within(drawer).getByRole('img', { name: `${student.name}的头像` })).toHaveAttribute(
      'src',
      avatarURL,
    );
    expect(within(drawer).queryByText('头像')).not.toBeInTheDocument();
    fireEvent.load(within(drawer).getByRole('img', { name: `${student.name}的头像` }));
    await waitFor(() => {
      expect(within(drawer).getByRole('button', { name: '预览头像' })).toBeEnabled();
    });
    await user.click(within(drawer).getByRole('button', { name: '预览头像' }));
    const preview = await screen.findByRole('dialog', { name: '头像预览' });
    expect(within(preview).getByRole('img', { name: `${student.name}的头像` })).toHaveAttribute(
      'src',
      avatarURL,
    );
    await user.keyboard('{Escape}');
    await waitFor(() => expect(preview).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe('/stu/roster');
    expect(new URLSearchParams(router.state.location.search).get('student_id')).toBe('1');
    await user.click(within(drawer).getByRole('button', { name: '关闭抽屉' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(router.state.location.pathname).toBe('/stu/roster');
    expect(router.state.location.search).toBe(`?workspace=${first}&source=fixture`);
    expect(screen.getByRole('searchbox', { name: '学号或姓名' })).toHaveValue('虚构');
    expect(screen.getByRole('table')).toBeVisible();
  });

  it.each([false, true])('合并编辑根据权限提交敏感字段：%s', async (allowed) => {
    const calls = mockAPI([
      'student.list',
      'student.update',
      ...(allowed ? [sensitiveAction] : []),
    ]);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const dialog = await screen.findByRole('dialog');
    const name = await within(dialog).findByLabelText('姓名 *');
    const tabs = within(dialog).getByRole('tablist', { name: '学生信息分区' });
    expect(tabs.closest('[data-slot="drawer-footer-navigation"]')).toBeInTheDocument();
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['学籍资料', '联系住宿', ...(allowed ? ['敏感信息'] : [])]);
    expect(within(dialog).getByText('2000-01-02')).toBeVisible();
    expect(within(dialog).getByText('由身份证号码生成')).toBeVisible();
    await userEvent.clear(name);
    await userEvent.type(name, '虚构学生修改');
    await userEvent.click(within(dialog).getByRole('combobox', { name: '状态' }));
    const status = await screen.findByRole('searchbox', { name: '搜索状态' });
    expect(status).toHaveAttribute('maxlength', '8');
    await userEvent.type(status, '待复核');
    await userEvent.click(screen.getByRole('option', { name: '新建“待复核”' }));
    if (allowed) {
      await userEvent.click(within(dialog).getByRole('tab', { name: '敏感信息' }));
      const address = within(dialog).getByLabelText('家庭详细住址');
      await userEvent.clear(address);
      await userEvent.type(address, '虚构地址2号');
      expect(within(dialog).getByLabelText('宗教信仰起始年份')).toHaveValue('2020年');
    } else {
      expect(within(dialog).queryByText('身份证号码')).not.toBeInTheDocument();
      expect(calls.some(({ url }) => url.endsWith('/sensitive'))).toBe(false);
    }
    expect(within(dialog).queryByRole('checkbox')).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    const saved = calls.find(({ options }) => options.method === 'PUT');
    expect(saved?.url).toBe(`/v1/workspaces/${first}/students/1`);
    expect(JSON.parse(typeof saved?.options.body === 'string' ? saved.options.body : '{}')).toEqual(
      {
        ...emptyStudent,
        ...Object.fromEntries(
          Object.keys(emptyStudent).map((key) => [key, student[key as keyof typeof student]]),
        ),
        name: '虚构学生修改',
        status: '待复核',
        revision: 1,
        ...(allowed
          ? {
              sensitive: sensitiveValues.map((item) => ({
                ...item,
                value: item.field === 'home_address' ? '虚构地址2号' : item.value,
              })),
            }
          : {}),
      },
    );
  });

  it('身份证号码变更后即时预览只读出生日期，清空时阻止保存', async () => {
    const calls = mockAPI(['student.list', 'student.update', sensitiveAction]);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const dialog = within(await screen.findByRole('dialog', { name: '编辑学生' }));
    await userEvent.click(dialog.getByRole('tab', { name: '敏感信息' }));
    const identity = await dialog.findByLabelText('身份证号码 *');
    expect(dialog.queryByRole('textbox', { name: '出生日期' })).not.toBeInTheDocument();
    await userEvent.clear(identity);
    await userEvent.type(identity, '00000020040229001x');
    await userEvent.click(dialog.getByRole('tab', { name: '学籍资料' }));
    expect(dialog.getByText('2004-02-29')).toBeVisible();
    expect(dialog.queryByText('2000-01-02')).not.toBeInTheDocument();
    await userEvent.click(dialog.getByRole('tab', { name: '敏感信息' }));
    await userEvent.clear(identity);
    await userEvent.click(dialog.getByRole('tab', { name: '学籍资料' }));
    await userEvent.click(dialog.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('身份证号码不能为空')).toBeVisible();
    expect(dialog.getByRole('tab', { name: '敏感信息' })).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => expect(identity).toHaveFocus());
    expect(identity).toHaveAttribute('aria-invalid', 'true');
    expect(calls.some(({ options }) => options.method === 'PUT')).toBe(false);
  });

  it('来校及既往教育信息使用月份选择器，支持修改和清空年月', async () => {
    const calls = mockAPI(['student.list', 'student.update']);
    const user = userEvent.setup();
    renderWorkspace();
    await user.click(await screen.findByRole('button', { name: '编辑' }));
    const editor = within(await screen.findByRole('dialog', { name: '编辑学生' }));
    const school = editor.getByLabelText('毕（肄）业学校及单位');
    expect(school).toHaveValue(student.previous_school_or_employer);
    await user.clear(school);
    await user.type(school, '虚构单位');

    await user.click(editor.getByRole('combobox', { name: '来校时间' }));
    const arrival = within(await screen.findByRole('dialog', { name: '来校时间选择' }));
    expect(arrival.getByLabelText('输入月份')).toHaveValue(student.arrival_month);
    await user.click(arrival.getByRole('button', { name: '2026年10月' }));
    expect(editor.getByRole('combobox', { name: '来校时间' })).toHaveTextContent('2026年10月');

    await user.click(editor.getByRole('combobox', { name: '毕（肄）业时间' }));
    const graduation = within(await screen.findByRole('dialog', { name: '毕（肄）业时间选择' }));
    await user.click(graduation.getByRole('button', { name: '清空' }));
    await user.click(editor.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      const body = calls.find(({ options }) => options.method === 'PUT')?.options.body;
      expect(typeof body === 'string' ? JSON.parse(body) : null).toMatchObject({
        arrival_month: '2026-10',
        previous_school_or_employer: '虚构单位',
        previous_graduation_month: '',
      });
    });
  });

  it('住宿类型留空时要求选择，编辑下拉只提供三个已知选项', async () => {
    const calls = mockAPI(['student.list', 'student.update'], '');
    renderWorkspace();
    await userEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const dialog = within(await screen.findByRole('dialog', { name: '编辑学生' }));
    await userEvent.click(dialog.getByRole('tab', { name: '联系住宿' }));
    const accommodation = dialog.getByRole('combobox', { name: '住宿类型' });
    expect(accommodation).toHaveTextContent('选择住宿类型');
    expect(accommodation).toHaveAttribute('aria-required', 'true');
    await userEvent.click(accommodation);
    const options = within(screen.getByRole('listbox', { name: '住宿类型' }));
    expect(options.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '出国联培',
      '校内',
      '校外',
    ]);
    await userEvent.click(options.getByRole('option', { name: '出国联培' }));
    await userEvent.click(dialog.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      const body = calls.find(({ options }) => options.method === 'PUT')?.options.body;
      expect(typeof body === 'string' ? JSON.parse(body) : null).toMatchObject({
        accommodation_type: 'joint_abroad',
      });
    });
  });

  it('编辑时显示当前头像，上传后刷新图片并使用新版本继续保存', async () => {
    const calls = mockAPI(['student.list', 'student.update', 'student.detail']);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const dialog = within(await screen.findByRole('dialog'));
    const image = await dialog.findByRole('img', { name: `${student.name}的头像` });
    expect(image).toHaveAttribute('src', avatarURL);
    expect(dialog.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    fireEvent.load(image);
    await waitFor(() => {
      expect(dialog.getByRole('button', { name: '预览头像' })).toBeEnabled();
    });
    await userEvent.click(dialog.getByRole('button', { name: '预览头像' }));
    const preview = await screen.findByRole('dialog', { name: '头像预览' });
    await userEvent.click(within(preview).getByRole('button', { name: '关闭预览' }));
    await waitFor(() => expect(preview).not.toBeInTheDocument());
    const file = new File(['fixture'], '头像.png', { type: 'image/png' });
    expect(dialog.queryByRole('button', { name: '选择文件' })).not.toBeInTheDocument();
    expect(dialog.getAllByRole('button', { name: '上传头像' })).toHaveLength(1);
    await userEvent.upload(dialog.getByLabelText('上传头像'), file);
    await waitFor(() => expect(dialog.getByRole('button', { name: '保存' })).toBeEnabled());
    expect(screen.getByText('上传完成')).toBeVisible();
    expect(dialog.getByRole('img', { name: `${student.name}的头像` })).toHaveAttribute(
      'src',
      updatedAvatarURL,
    );
    expect(dialog.getByRole('status', { name: '正在加载头像' })).toBeVisible();
    fireEvent.load(dialog.getByRole('img', { name: `${student.name}的头像` }));
    await waitFor(() => {
      expect(dialog.queryByRole('status', { name: '正在加载头像' })).not.toBeInTheDocument();
    });
    const upload = calls.find(({ url }) => url.endsWith('/1/avatar'));
    expect(upload?.options.method).toBe('POST');
    expect(JSON.parse(typeof upload?.options.body === 'string' ? upload.options.body : '')).toEqual(
      {
        filename: file.name,
        size: file.size,
        content_type: 'image/png',
        revision: 1,
      },
    );
    expect(calls.some(({ url }) => url.endsWith('/1/avatar/complete'))).toBe(true);
    await userEvent.click(dialog.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      const body = calls.find(({ options }) => options.method === 'PUT')?.options.body;
      expect(typeof body === 'string' ? JSON.parse(body) : null).toMatchObject({ revision: 2 });
    });
  });

  it('删除未完成头像时只显示删除按钮的加载状态', async () => {
    let finishDeletion: (response: Response) => void = () => {
      throw new Error('尚未请求删除头像');
    };
    const deletion = new Promise<Response>((resolve) => {
      finishDeletion = resolve;
    });
    const request = vi.fn((_url: string, options: RequestInit) =>
      options.method === 'DELETE'
        ? deletion
        : Promise.resolve(
            respond({
              ...student,
              avatar_url: '',
              avatar_uploads: [
                { id: 7, filename: '未完成头像.png', state: 'pending', revision: 1 },
              ],
            }),
          ),
    );
    vi.stubGlobal('fetch', request);
    render(
      <QueryClientProvider client={queryClient()}>
        <StudentEditor
          workspaceID={first}
          student={student}
          canBasic
          canDetail
          canQuery={false}
          canSensitive={false}
          onClose={vi.fn()}
        />
      </QueryClientProvider>,
    );
    const dialog = within(screen.getByRole('dialog', { name: '编辑学生' }));
    const remove = await dialog.findByRole('button', { name: '删除' });
    await userEvent.click(remove);
    const upload = dialog.getByRole('button', { name: '上传头像' });
    expect(remove).toHaveAttribute('aria-busy', 'true');
    expect(upload).toBeDisabled();
    expect(upload).not.toHaveAttribute('aria-busy');
    expect(screen.queryByRole('group', { name: '文件上传状态' })).not.toBeInTheDocument();
    finishDeletion(respond(null));
    await waitFor(() => expect(upload).toBeEnabled());
  });

  it('只有处理敏感信息权限时，编辑只展示敏感表单并全量保存', async () => {
    const calls = mockAPI(['student.list', sensitiveAction]);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const dialog = await screen.findByRole('dialog');
    const address = await within(dialog).findByLabelText('家庭详细住址');
    expect(within(dialog).queryByLabelText('学号 *')).not.toBeInTheDocument();
    expect(within(dialog).queryByLabelText('头像文件')).not.toBeInTheDocument();
    await userEvent.clear(address);
    await userEvent.click(within(dialog).getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    const saved = calls.find(({ options }) => options.method === 'PUT');
    expect(saved?.url).toBe(`/v1/workspaces/${first}/students/1/sensitive`);
    expect(JSON.parse(typeof saved?.options.body === 'string' ? saved.options.body : '{}')).toEqual(
      {
        revision: 1,
        values: sensitiveValues.map((item) => ({
          ...item,
          value: item.field === 'home_address' ? '' : item.value,
        })),
      },
    );
  });

  it('四张统计卡在筛选上方，列表顺序固定且支持横向滚动', async () => {
    mockAPI(['student.list', 'student.statistics']);
    renderWorkspace();
    await screen.findByText(student.name);
    const charts = screen.getByLabelText('学生统计');
    for (const label of ['学生数量', '班级数量', '有宗教信仰', '校外住宿'])
      expect(within(charts).getByText(label)).toBeVisible();
    expect(within(charts).getByText('12')).toBeVisible();
    expect(
      charts.compareDocumentPosition(screen.getByRole('searchbox', { name: '学号或姓名' })),
    ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.getAllByRole('columnheader').map((item) => item.textContent)).toEqual([
      '姓名',
      '学号',
      '年级',
      '状态',
      '导师',
      '专业',
      '学历',
      '入学政治面貌',
      '住宿园区',
      '宿舍楼',
      '宿舍号',
      '电话',
      '创建时间',
      '更新时间',
      '操作',
    ]);
    expect(screen.getByRole('columnheader', { name: '姓名' })).toHaveClass(
      'table-pinned',
      'left-0',
    );
    const nameCell = screen.getByRole('cell', { name: student.name });
    expect(nameCell).toHaveClass('table-pinned', 'left-0');
    expect(within(nameCell).queryByRole('link')).not.toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: '学号' })).not.toHaveClass('table-pinned');
    expect(screen.getByRole('region', { name: '表格内容，可横向滚动' })).toHaveClass(
      'overflow-x-auto',
    );
    expect(screen.getByRole('table')).toHaveClass('min-w-full');
    expect(screen.getByRole('navigation', { name: '学生工作导航' })).toBeVisible();
  });

  it('学生信息保留三行常用筛选，默认状态始终可见，展开后可修改', async () => {
    mockAPI(['student.list']);
    renderWorkspace();
    await screen.findByText(student.name);
    const labels = [
      '学历',
      '年级',
      '班级',
      '班内任职',
      '住宿类型',
      '住宿园区',
      '宿舍楼',
      '宿舍号',
      '民族',
      '导师姓名',
      '是否有宗教信仰',
    ];
    const fields = labels.map((name) => screen.getByRole('combobox', { name }));
    for (const field of fields) expect(field).toBeVisible();
    fields.slice(1).forEach((field, index) => {
      expect(fields[index]?.compareDocumentPosition(field)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    });
    expect(screen.queryByRole('combobox', { name: '状态' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: '专业' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '移除筛选 状态：在籍' })).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: '更多筛选（1）' }));
    expect(screen.getAllByRole('combobox', { name: '状态' })).toHaveLength(1);
    expect(screen.getByRole('combobox', { name: '专业' })).toBeVisible();
    expect(screen.getByRole('combobox', { name: '入学政治面貌' })).toBeVisible();
  });

  it('状态默认筛选在籍，固定及动态枚举变化后立即请求当前空间的数据', async () => {
    const calls = mockAPI(['student.list', sensitiveAction]);
    renderWorkspace();
    await screen.findByText(student.name);
    expect(screen.getByLabelText('已应用筛选')).toHaveTextContent('状态：在籍');
    const initialList = calls.find(({ url }) =>
      url.startsWith(`/v1/workspaces/${first}/students?`),
    );
    const initialFilters = new URL(initialList?.url ?? '', 'https://fixture.example').searchParams;
    expect(initialFilters.get('status')).toBe('在籍');
    await userEvent.click(screen.getByRole('button', { name: /更多筛选/u }));
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveTextContent('在籍');
    for (const [label, value] of [
      ['年级', '2026级'],
      ['状态', student.status],
    ] as const) {
      await userEvent.click(screen.getByRole('combobox', { name: label }));
      await userEvent.click(await screen.findByRole('option', { name: value }));
    }
    expect(screen.getAllByRole('combobox', { name: '年级' })).toHaveLength(1);
    expect(screen.getAllByRole('combobox', { name: '状态' })).toHaveLength(1);
    expect(screen.queryByRole('combobox', { name: '宗教信仰起始年份' })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('combobox', { name: '学历' }));
    await userEvent.click(await screen.findByRole('option', { name: '博士' }));
    await userEvent.click(screen.getByRole('combobox', { name: '是否有宗教信仰' }));
    await userEvent.click(await screen.findByRole('option', { name: '是' }));
    await waitFor(() => {
      expect(
        calls.some(
          ({ url }) =>
            new URL(url, 'https://fixture.example').searchParams.get('grade') === '2026级' &&
            new URL(url, 'https://fixture.example').searchParams
              .getAll('status')
              .includes(student.status) &&
            url.includes('education_level='),
        ),
      ).toBe(true);
    });
    expect(calls.some(({ url }) => url.endsWith('/sensitive-options?field=has_religion'))).toBe(
      true,
    );
  });

  it('更多筛选收起后保留已应用提示，状态可重新选择在籍', async () => {
    const calls = mockAPI(['student.list']);
    renderWorkspace();
    await screen.findByText(student.name);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /更多筛选/u }));
    await user.click(screen.getByRole('combobox', { name: '状态' }));
    await user.click(await screen.findByRole('option', { name: '全部状态' }));
    await user.keyboard('{Escape}');
    expect(screen.queryByLabelText('已应用筛选')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: '状态' }));
    await user.click(await screen.findByRole('option', { name: '在籍' }));
    await user.keyboard('{Escape}');
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveTextContent('在籍');
    await user.click(screen.getByRole('combobox', { name: '班内任职状态' }));
    await user.click(screen.getByRole('option', { name: '有任职' }));
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: '更多筛选（2）' }));
    expect(screen.queryByRole('combobox', { name: '班内任职状态' })).not.toBeInTheDocument();
    const applied = screen.getByLabelText('已应用筛选');
    expect(applied).toHaveTextContent('状态：在籍');
    expect(applied).toHaveTextContent('班内任职状态：有任职');
    expect(calls.filter(({ url }) => url.includes('/students?')).at(-1)?.url).toContain(
      'position_status=assigned',
    );
  });

  it('撤销筛选保留文字草稿，统一通过重置恢复在籍', async () => {
    const calls = mockAPI(['student.list', 'student.statistics']);
    renderWorkspace();
    await screen.findByText(student.name);
    const user = userEvent.setup();
    const keyword = screen.getByRole('searchbox', { name: '学号或姓名' });
    await user.type(keyword, '待搜索文字');
    await user.click(screen.getByRole('button', { name: '移除筛选 状态：在籍' }));
    expect(keyword).toHaveValue('待搜索文字');
    let url = new URL(
      calls.filter(({ url }) => url.includes('/students?')).at(-1)?.url ?? '',
      'https://fixture.example',
    );
    expect(url.searchParams.has('status')).toBe(false);
    expect(url.searchParams.get('keyword')).toBeNull();
    await user.click(screen.getByRole('button', { name: '搜索' }));
    expect(screen.getByLabelText('已应用筛选')).toHaveTextContent('学号或姓名：待搜索文字');
    expect(screen.queryByRole('button', { name: '清除筛选' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '重置' }));
    expect(keyword).toHaveValue('');
    expect(screen.getByLabelText('已应用筛选')).toHaveTextContent('状态：在籍');
    url = new URL(
      calls.filter(({ url }) => url.includes('/students?')).at(-1)?.url ?? '',
      'https://fixture.example',
    );
    expect(url.searchParams.get('status')).toBe('在籍');
  });

  it('学生姓名可打开详情，更多菜单保留删除确认且取消不写入', async () => {
    const calls = mockAPI(['student.list', 'student.detail', 'student.delete']);
    const router = renderWorkspace({ source: 'fixture' }, '#student-list');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('link', {
        name: `查看 ${student.name}(${student.student_no}) 的详情`,
      }),
    );
    expect(await screen.findByRole('dialog', { name: '学生详情' })).toBeVisible();
    expect(router.state.location.search).toContain('source=fixture');
    expect(router.state.location.hash).toBe('#student-list');
    await user.click(screen.getByRole('button', { name: '关闭抽屉' }));
    expect(router.state.location.hash).toBe('#student-list');
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    await user.click(
      screen.getByRole('button', { name: `更多操作 ${student.name}(${student.student_no})` }),
    );
    await user.click(screen.getByRole('menuitem', { name: '删除' }));
    const confirmation = within(await screen.findByRole('dialog', { name: '删除学生' }));
    expect(confirmation.getByText(/将永久删除/)).toBeVisible();
    await user.click(confirmation.getByRole('button', { name: '取消' }));
    expect(calls.some(({ options }) => options.method === 'DELETE')).toBe(false);
  });

  it('编辑分区保留草稿并统一保存，错误字段自动定位到学籍资料', async () => {
    const calls = mockAPI(['student.list', 'student.update']);
    renderWorkspace();
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: '编辑' }));
    const editor = within(await screen.findByRole('dialog', { name: '编辑学生' }));
    const name = editor.getByLabelText('姓名 *');
    await user.clear(name);
    await user.click(editor.getByRole('tab', { name: '联系住宿' }));
    const phone = editor.getByLabelText('电话号码');
    await user.clear(phone);
    await user.type(phone, '虚构新电话');
    await user.click(editor.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('姓名不能为空')).toBeVisible();
    expect(editor.getByRole('tab', { name: '学籍资料' })).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => expect(name).toHaveFocus());
    expect(calls.some(({ options }) => options.method === 'PUT')).toBe(false);
    await user.click(editor.getByLabelText('学号 *'));
    await user.click(editor.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(name).toHaveFocus());
    expect(calls.some(({ options }) => options.method === 'PUT')).toBe(false);
    await user.type(name, '虚构新姓名');
    await user.click(editor.getByRole('tab', { name: '联系住宿' }));
    expect(phone).toHaveValue('虚构新电话');
    await user.click(editor.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    const body = calls.find(({ options }) => options.method === 'PUT')?.options.body;
    expect(typeof body === 'string' ? JSON.parse(body) : null).toMatchObject({
      name: '虚构新姓名',
      phone: '虚构新电话',
    });
  });

  it('切换工作空间时不复用上一空间的学生', async () => {
    let resolveSecond: (value: Response) => void = () => {
      throw new Error('第二空间尚未请求');
    };
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) =>
        url.includes(first)
          ? Promise.resolve(respond(listing()))
          : new Promise<Response>((resolve) => {
              resolveSecond = resolve;
            }),
      ),
    );
    const client = queryClient();
    const view = (id: string) => (
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <StudentPage
            key={id}
            workspaceID={id}
            workspaceName="虚构空间"
            actions={['student.list']}
            onRefresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>
    );
    const rendered = render(view(first));
    await screen.findByText(student.name);
    rendered.rerender(view(second));
    expect(screen.queryByText(student.name)).not.toBeInTheDocument();
    resolveSecond(respond({ ...listing(), items: [{ ...student, id: 2, name: '虚构学生乙' }] }));
    expect(await screen.findByText('虚构学生乙')).toBeVisible();
  });
});
