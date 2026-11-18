import { useState } from 'react';

import { getConfig } from '@/lib/config';

export function useTablePagination() {
  const [pagination, setPagination] = useState(() => ({
    page: 1,
    pageSize: getConfig().tablePageSize,
  }));
  return {
    ...pagination,
    setPage: (page: number) => {
      setPagination((current) => ({ ...current, page }));
    },
    setPageSize: (pageSize: number) => {
      setPagination({ page: 1, pageSize });
    },
  };
}
