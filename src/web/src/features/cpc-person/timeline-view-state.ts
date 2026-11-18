import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import type { PersonFilters } from '@/features/cpc-person/api';
import { personTimelineTemplateOptions } from '@/features/cpc-person/timeline-api';
import {
  timelineViewListOptions,
  timelineViewOptions,
  timelineViewStatisticsOptions,
} from '@/features/cpc-person/timeline-view-api';
import { ApiError } from '@/lib/api';
import { useTablePagination } from '@/lib/pagination';

const denied = (error: Error | null) =>
  error instanceof ApiError && [401, 403, 404].includes(error.status);

export function usePersonTimelineView({
  workspaceID,
  filters,
  active,
  canList,
  canStatistics,
}: {
  workspaceID: string;
  filters: PersonFilters;
  active: boolean;
  canList: boolean;
  canStatistics: boolean;
}) {
  const client = useQueryClient();
  const pagination = useTablePagination();
  const [keyword, setKeyword] = useState('');
  const [chosenTemplate, setChosenTemplate] = useState<{ value: string; label: string } | null>(
    null,
  );
  const [nodeID, setNodeID] = useState(0);
  const templates = useQuery({
    ...personTimelineTemplateOptions(workspaceID, 'list', keyword),
    enabled: active && canList,
  });
  const defaultTemplate = client.getQueryData(
    personTimelineTemplateOptions(workspaceID, 'list', '').queryKey,
  )?.[0];
  const selectedTemplate = chosenTemplate ?? templates.data?.[0];
  const templateID = Number(selectedTemplate?.value ?? 0);
  const accessible = canList && !denied(templates.error);
  const template = useQuery({
    ...timelineViewOptions(workspaceID, templateID),
    enabled: active && accessible && templateID > 0,
  });
  const nodes = accessible && !denied(template.error) ? (template.data?.nodes ?? []) : [];
  const node = nodes.find((item) => item.numeric_id === nodeID) ?? nodes[0];
  const selectedID = node?.numeric_id ?? 0;
  const ready = accessible && templateID > 0 && selectedID > 0;
  const query = useQuery({
    ...timelineViewListOptions(
      workspaceID,
      templateID,
      selectedID,
      filters,
      pagination.page,
      pagination.pageSize,
    ),
    enabled: active && ready,
    // 只在同一模板及节点重查时保留结果，避免旧字段与新节点的列混排。
    placeholderData: (previous, previousQuery) =>
      previousQuery?.queryKey[4] === templateID && previousQuery.queryKey[5] === selectedID
        ? previous
        : undefined,
  });
  const statistics = useQuery({
    ...timelineViewStatisticsOptions(workspaceID, templateID, selectedID, filters),
    enabled: active && ready && canStatistics,
    placeholderData: (previous, previousQuery) =>
      previousQuery?.queryKey[4] === templateID && previousQuery.queryKey[5] === selectedID
        ? previous
        : undefined,
  });
  const data = ready && !denied(query.error) ? query.data : undefined;
  const stats = ready && canStatistics && !denied(statistics.error) ? statistics.data : undefined;
  const options = [...(denied(templates.error) ? [] : (templates.data ?? []))];
  if (
    selectedTemplate &&
    accessible &&
    !options.some((item) => item.value === selectedTemplate.value)
  ) {
    options.unshift(selectedTemplate);
  }

  return {
    templates,
    template,
    templateID,
    defaultTemplateValue: defaultTemplate?.value ?? '',
    node,
    nodes,
    query,
    statistics,
    data,
    stats,
    pagination,
    canStatistics,
    options,
    loading:
      active &&
      canList &&
      (templates.isPending ||
        (templateID > 0 && template.isPending) ||
        (ready && (query.isPending || (canStatistics && statistics.isPending)))),
    searchTemplates: (value: string) => {
      // 搜索选项时保留当前选择，防止搜索结果改变默认模板。
      if (selectedTemplate) setChosenTemplate(selectedTemplate);
      setKeyword(value);
    },
    selectTemplate: (value: string) => {
      const option =
        options.find((item) => item.value === value) ??
        (defaultTemplate?.value === value ? defaultTemplate : undefined);
      if (!option || value === String(templateID)) return;
      setChosenTemplate(option);
      setNodeID(0);
      pagination.setPage(1);
    },
    selectNode: (value: number) => {
      setNodeID(value);
      pagination.setPage(1);
    },
    fetchPage: (page: number, size: number, values = filters) =>
      ready
        ? client.query({
            ...timelineViewListOptions(workspaceID, templateID, selectedID, values, page, size),
            staleTime: 0,
          })
        : Promise.resolve(),
    fetchStatistics: (values: PersonFilters) =>
      ready && canStatistics
        ? client.query({
            ...timelineViewStatisticsOptions(workspaceID, templateID, selectedID, values),
            staleTime: 0,
          })
        : Promise.resolve(),
  };
}
