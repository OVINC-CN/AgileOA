import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Graduation, GraduationInput } from '@/features/graduation/api';
import { GraduationEditor } from '@/features/graduation/graduation-editor';
import { GraduationPage } from '@/features/graduation/graduation-page';
import { buildGraduationSeries } from '@/features/graduation/series';

const first = 'w7m2k9p4s6n8c3x5';
const second = 'r4v8b2m6k9d3p5s7';
const savedType = `custom_${'a'.repeat(64)}`;
const record: Graduation = {
  student_id: 1,
  student_no: 'S20260001',
  student_name: '虚构学生',
  education_level: '本科',
  major: '虚构专业',
  grade: '2026级',
  graduation_year: '',
  type: '',
  type_label: '',
  destination: '',
  revision: 1,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const client = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
const respond = (data: unknown) =>
  new Response(JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }));
afterEach(() => {
  vi.unstubAllGlobals();
});

describe('毕业去向', () => {
  it('按已填写的毕业年份堆叠各维度人数，缺失组合补零但不生成就业率', () => {
    const series = buildGraduationSeries([
      {
        graduation_year: '2026',
        value: 'major.a',
        label: '专业.甲[0]',
        total: 2,
        employed: 1,
        pending: 1,
        employment_rate: 0.5,
      },
      {
        graduation_year: '2026',
        value: '__proto__',
        label: '专业乙',
        total: 3,
        employed: 3,
        pending: 0,
        employment_rate: 1,
      },
      {
        graduation_year: '2027',
        value: 'major.a',
        label: '专业.甲[0]',
        total: 1,
        employed: 0,
        pending: 1,
        employment_rate: 0,
      },
      {
        graduation_year: '',
        value: 'major.c',
        label: '专业丙',
        total: 1,
        employed: 0,
        pending: 1,
        employment_rate: 0,
      },
    ]);
    const firstCategory = series.categories.find((item) => item.value === 'major.a');
    const secondCategory = series.categories.find((item) => item.value === '__proto__');
    expect(series.rows.map((row) => row['year'])).toEqual(['2026', '2027']);
    expect(series.categories.map((item) => item.value)).not.toContain('major.c');
    expect(series.rows[0]?.[firstCategory?.countKey ?? '']).toBe(2);
    expect(series.rows[0]?.[secondCategory?.countKey ?? '']).toBe(3);
    expect(series.rows[0]?.[firstCategory?.rateKey ?? '']).toBe(0.5);
    expect(series.rows[1]?.[secondCategory?.countKey ?? '']).toBe(0);
    expect(series.rows[1]?.[secondCategory?.rateKey ?? '']).toBeNull();
  });
  it('待就业只对应空类型，不发送独立的待就业枚举', async () => {
    let body: unknown;
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, options: RequestInit) => {
        body = JSON.parse(typeof options.body === 'string' ? options.body : 'null') as unknown;
        return Promise.resolve(respond({ student_id: 1, revision: 2 }));
      }),
    );
    render(
      <QueryClientProvider client={client()}>
        <GraduationEditor
          workspaceID={first}
          record={record}
          types={[]}
          onClose={() => undefined}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('combobox', { name: '类型' })).toHaveTextContent('待就业');
    await userEvent.type(screen.getByRole('spinbutton', { name: '毕业年份' }), '2026');
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(body).toEqual({
        graduation_year: '2026',
        type: '',
        type_label: '',
        destination: '',
        revision: 1,
      });
    });
  });

  it('可新建类型名称，选择待就业后恢复为空值', async () => {
    const bodies: GraduationInput[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, options: RequestInit) => {
        bodies.push(
          JSON.parse(typeof options.body === 'string' ? options.body : '') as GraduationInput,
        );
        return Promise.resolve(respond({ student_id: 1, revision: 2 }));
      }),
    );
    render(
      <QueryClientProvider client={client()}>
        <GraduationEditor
          workspaceID={first}
          record={record}
          types={[]}
          onClose={() => undefined}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '类型' }));
    await user.type(screen.getByRole('searchbox', { name: '搜索类型' }), '签约就业');
    await user.click(screen.getByRole('option', { name: '新建“签约就业”' }));
    await user.type(screen.getByRole('textbox', { name: '毕业去向' }), '虚构单位');
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(bodies[0]).toMatchObject({
        type: '',
        type_label: '签约就业',
        destination: '虚构单位',
      });
    });
    await user.click(screen.getByRole('combobox', { name: '类型' }));
    await user.click(screen.getByRole('option', { name: '待就业' }));
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(bodies[1]).toMatchObject({ type: '', type_label: '', destination: '虚构单位' });
    });
  });

  it.each([
    { type: null, label: '全部类型' },
    { type: '', label: '待就业' },
    { type: savedType, label: '签约就业' },
    { type: ['', savedType], label: '待就业和签约就业' },
  ])('选项变化立即请求类型 $label，连续选择保留当前空间表格', async (selected) => {
    const requests: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        requests.push(path);
        if (path.includes('/filter-options'))
          return Promise.resolve(
            respond({
              education_levels: [{ value: '本科', label: '本科' }],
              majors: [{ value: '虚构专业', label: '虚构专业' }],
              grades: [{ value: '2026级', label: '2026级' }],
              graduation_years: [{ value: '2026', label: '2026' }],
              types: [{ value: savedType, label: '签约就业' }],
            }),
          );
        if (new URL(path, 'https://fixture.example').searchParams.has('grade'))
          return new Promise<Response>(() => undefined);
        return Promise.resolve(respond({ items: [record], total: 1, page: 1, page_size: 5 }));
      }),
    );
    render(
      <QueryClientProvider client={client()}>
        <GraduationPage
          workspaceID={first}
          workspaceName="虚构空间"
          actions={['graduation.list']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    expect(await screen.findByText('虚构学生')).toBeInTheDocument();
    const user = userEvent.setup();
    for (const [label, value] of [
      ['学历', '本科'],
      ['专业', '虚构专业'],
      ['年级', '2026级'],
      ['毕业年份', '2026'],
      ['类型', selected.label],
    ] as const) {
      await user.click(screen.getByRole('combobox', { name: label }));
      const choices =
        label === '类型' && Array.isArray(selected.type) ? ['待就业', '签约就业'] : [value];
      for (const choice of choices) {
        await user.click(screen.getByRole('option', { name: choice }));
      }
      await user.keyboard('{Escape}');
    }
    await waitFor(() => {
      expect(
        requests.some((path) => {
          const params = new URL(path, 'https://fixture.example').searchParams;
          return (
            params.get('education_level') === '本科' &&
            params.get('major') === '虚构专业' &&
            params.get('grade') === '2026级' &&
            params.get('graduation_year') === '2026' &&
            JSON.stringify(params.getAll('type')) ===
              JSON.stringify(
                selected.type === null
                  ? []
                  : Array.isArray(selected.type)
                    ? selected.type
                    : [selected.type],
              )
          );
        }),
      ).toBe(true);
    });
    expect(screen.getByText('虚构学生')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '搜索' }));
    expect(screen.getByRole('button', { name: '搜索' })).toHaveAttribute('aria-busy', 'true');
  });

  it('切换工作空间后不展示上一空间的学生或去向', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        if (path.includes(second)) return new Promise<Response>(() => undefined);
        if (path.includes('/filter-options'))
          return Promise.resolve(
            respond({
              education_levels: [],
              majors: [],
              grades: [],
              graduation_years: [],
              types: [],
            }),
          );
        return Promise.resolve(respond({ items: [record], total: 1, page: 1, page_size: 5 }));
      }),
    );
    const cache = client();
    const props = {
      workspaceID: first,
      workspaceName: '虚构空间甲',
      actions: ['graduation.list'],
      refresh: () => Promise.resolve(),
    };
    const view = render(
      <QueryClientProvider client={cache}>
        <GraduationPage {...props} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText('虚构学生')).toBeInTheDocument();
    view.rerender(
      <QueryClientProvider client={cache}>
        <GraduationPage {...props} workspaceID={second} workspaceName="虚构空间乙" />
      </QueryClientProvider>,
    );
    expect(screen.queryByText('虚构学生')).not.toBeInTheDocument();
  });
});
