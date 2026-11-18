import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, MemoryRouter, RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppLayout } from '@/components/app-layout';
import { emptyStudent } from '@/features/student/api';
import { birthDateFromIdentityNumber, sensitiveAction } from '@/features/student/fields';
import { StudentAvatar } from '@/features/student/student-avatar';
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
function renderWorkspace(query: Record<string, string> = {}) {
  const search = new URLSearchParams({ workspace: first, ...query });
  const router = createMemoryRouter(
    [
      {
        path: '/stu',
        Component: WorkspacePage,
        children: [{ path: 'roster', Component: WorkspaceStudentsPage }],
      },
    ],
    { initialEntries: [`/stu/roster?${search.toString()}`] },
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
    const position = screen.getByRole('combobox', { name: '班内任职' });
    expect(screen.queryByRole('combobox', { name: '班内任职状态' })).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: '是否有宗教信仰' })).toBeDisabled();
    await userEvent.click(screen.getByRole('button', { name: '更多筛选' }));
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

  it.each([false, true])('详情页面敏感信息默认折叠，显示原始值权限：%s', async (allowed) => {
    const calls = mockAPI([
      'student.list',
      'student.detail',
      ...(allowed ? [sensitiveAction] : []),
    ]);
    renderWorkspace();
    await userEvent.click(await screen.findByRole('link', { name: '详情' }));
    const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
    await drawer.findByRole('heading', { name: '基本信息' });
    for (const name of ['学籍信息', '导师信息', '个人信息', '联系方式', '住宿信息', '家庭联系'])
      expect(screen.getByRole('heading', { name })).toBeVisible();
    expect(drawer.getByText(student.previous_school_or_employer)).toBeVisible();
    expect(drawer.getByText(student.arrival_month)).toBeVisible();
    expect(drawer.getByText(student.previous_graduation_month)).toBeVisible();
    const academic = drawer.getByRole('button', { name: '学籍信息' });
    await userEvent.click(academic);
    expect(academic).toHaveAttribute('aria-expanded', 'false');
    expect(drawer.getByText(student.advisor_no)).toBeVisible();
    const sensitiveSection = drawer.getByRole('button', { name: '敏感信息' });
    expect(sensitiveSection).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('宗教信仰起始年份')).not.toBeInTheDocument();
    expect(calls.some(({ url }) => url.endsWith('/sensitive'))).toBe(false);
    expect(screen.getByText('虚构户籍市')).toBeVisible();
    expect(screen.getByText('虚构籍贯县')).toBeVisible();
    if (!allowed) {
      expect(drawer.queryByRole('button', { name: '显示原始值' })).not.toBeInTheDocument();
      await userEvent.click(sensitiveSection);
      expect(screen.getAllByText('***')).toHaveLength(14);
      return;
    }
    const reveal = drawer.getByRole('button', { name: '显示原始值' });
    expect(reveal.parentElement).toBe(drawer.getByRole('button', { name: '刷新' }).parentElement);
    await userEvent.click(reveal);
    expect(sensitiveSection).toHaveAttribute('aria-expanded', 'true');
    expect(await screen.findByText(fixtureIdentityNumber)).toBeVisible();
    expect(screen.getByText('2020年')).toBeVisible();
    await userEvent.click(drawer.getByRole('button', { name: '隐藏原始值' }));
    expect(sensitiveSection).toHaveAttribute('aria-expanded', 'false');
    await userEvent.click(sensitiveSection);
    expect(screen.getAllByText('***')).toHaveLength(14);
    expect(drawer.queryByRole('button', { name: '复制身份证号码' })).not.toBeInTheDocument();
  });

  it('校外住宿并入普通编辑，详情字段可复制', async () => {
    const calls = mockAPI(['student.list', 'student.detail', 'student.update'], 'off_campus');
    renderWorkspace();
    const user = userEvent.setup();
    const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    await user.click(await screen.findByRole('link', { name: '详情' }));
    const drawer = within(await screen.findByRole('dialog', { name: '学生详情' }));
    const address = await drawer.findByText('校外居住地详细地址');
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
      await drawer.findByRole('heading', { name: '基本信息' });
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
    const section = within(await drawer.findByRole('region', { name: '住宿信息' }));
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
    expect(await within(drawer).findByRole('heading', { name: '基本信息' })).toBeVisible();
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
    const identity = await dialog.findByLabelText('身份证号码 *');
    expect(dialog.queryByRole('textbox', { name: '出生日期' })).not.toBeInTheDocument();
    await userEvent.clear(identity);
    await userEvent.type(identity, '00000020040229001x');
    expect(dialog.getByText('2004-02-29')).toBeVisible();
    expect(dialog.queryByText('2000-01-02')).not.toBeInTheDocument();
    await userEvent.clear(identity);
    await userEvent.click(dialog.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('身份证号码不能为空')).toBeVisible();
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
      '学号',
      '姓名',
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
    expect(screen.getByRole('region', { name: '表格内容，可横向滚动' })).toHaveClass(
      'overflow-x-auto',
    );
    expect(screen.getByRole('table')).toHaveClass('min-w-full');
    expect(screen.getByRole('navigation', { name: '学生工作导航' })).toBeVisible();
  });

  it('学历和是否有宗教信仰默认显示，学历位于年级前面', async () => {
    mockAPI(['student.list']);
    renderWorkspace();
    await screen.findByText(student.name);
    const names = [
      '学历',
      '年级',
      '住宿园区',
      '宿舍楼',
      '宿舍号',
      '民族',
      '导师姓名',
      '是否有宗教信仰',
    ];
    for (const name of names) expect(screen.getByRole('combobox', { name })).toBeVisible();
    const education = screen.getByRole('combobox', { name: '学历' });
    const grade = screen.getByRole('combobox', { name: '年级' });
    expect(education.compareDocumentPosition(grade)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);

    await userEvent.click(screen.getByRole('button', { name: '更多筛选' }));
    for (const name of names) expect(screen.getAllByRole('combobox', { name })).toHaveLength(1);
    expect(screen.getByRole('combobox', { name: '入学政治面貌' })).toBeVisible();
  });

  it('状态默认筛选在籍，固定及动态枚举变化后立即请求当前空间的数据', async () => {
    const calls = mockAPI(['student.list', sensitiveAction]);
    renderWorkspace();
    await screen.findByText(student.name);
    expect(screen.queryByRole('combobox', { name: '状态' })).not.toBeInTheDocument();
    const initialList = calls.find(({ url }) =>
      url.startsWith(`/v1/workspaces/${first}/students?`),
    );
    const initialFilters = new URL(initialList?.url ?? '', 'https://fixture.example').searchParams;
    expect(initialFilters.get('status')).toBe('在籍');
    await userEvent.click(screen.getByRole('button', { name: '更多筛选' }));
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
