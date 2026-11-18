import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { filterParams } from '@/lib/filter-params';

export const workspaceIDSchema = z.string().regex(/^(?=.*[a-z])(?=.*[0-9])[a-z0-9]{16}$/);

const workspaceSchema = z.object({
  id: workspaceIDSchema,
  name: z.string().min(1),
  description: z.string(),
  is_enabled: z.boolean(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type Workspace = z.infer<typeof workspaceSchema>;
const workspaceListSchema = z.object({
  items: z.array(workspaceSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});
export interface WorkspaceFilters {
  keyword: string;
  status: string[];
}

export const workspaceListOptions = (filters: WorkspaceFilters, page: number, pageSize: number) =>
  queryOptions({
    queryKey: ['management', 'workspaces', 'list', filters, page, pageSize],
    queryFn: ({ signal }) => {
      const params = filterParams({
        ...filters,
        page: String(page),
        page_size: String(pageSize),
      });
      return apiGet(`/v1/system/workspaces?${params.toString()}`, workspaceListSchema, signal);
    },
  });

export const workspaceDetailOptions = (id: string) =>
  queryOptions({
    queryKey: ['management', 'workspaces', 'detail', id],
    queryFn: ({ signal }) => apiGet(`/v1/system/workspaces/${id}`, workspaceSchema, signal),
    enabled: id !== '',
  });

export const workspaceEnumOptions = queryOptions({
  queryKey: ['management', 'workspaces', 'enum'],
  queryFn: ({ signal }) =>
    apiGet(
      '/v1/system/workspaces/enum',
      z.array(
        z.object({
          value: workspaceIDSchema,
          label: z.string().min(1),
          is_enabled: z.boolean(),
        }),
      ),
      signal,
    ),
});

export function createWorkspace(values: { name: string; description: string }) {
  return apiPost('/v1/system/workspaces', workspaceSchema, values);
}

export function updateWorkspace(
  workspace: Workspace,
  values: { name: string; description: string; is_enabled: boolean },
) {
  return apiPut(`/v1/system/workspaces/${workspace.id}`, workspaceSchema, {
    ...values,
    revision: workspace.revision,
  });
}

export function deleteWorkspace(id: string) {
  return apiDelete(`/v1/system/workspaces/${id}`, z.null());
}

export const myWorkspacesOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ['workspace', 'mine', page, pageSize],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/workspaces?page=${String(page)}&page_size=${String(pageSize)}`,
        workspaceListSchema,
        signal,
      ),
    staleTime: 0,
  });

export const myWorkspacePagesOptions = (pageSize: number) =>
  infiniteQueryOptions({
    queryKey: ['workspace', 'mine', 'pages', pageSize],
    initialPageParam: 1,
    queryFn: ({ signal, pageParam }) =>
      apiGet(
        `/v1/workspaces?page=${String(pageParam)}&page_size=${String(pageSize)}`,
        workspaceListSchema,
        signal,
      ),
    getNextPageParam: (lastPage) =>
      lastPage.page * lastPage.page_size < lastPage.total ? lastPage.page + 1 : undefined,
    staleTime: 0,
  });

export const workspaceContextOptions = (id: string) =>
  queryOptions({
    queryKey: ['workspace', id, 'context'],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/workspaces/${id}`,
        z.object({
          workspace: workspaceSchema,
          actions: z.array(z.string()),
          can_review_project_applications: z.boolean().default(false),
        }),
        signal,
      ),
    enabled: workspaceIDSchema.safeParse(id).success,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
  });
