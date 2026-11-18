import { useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router';

const tabQueryParam = 'tab';

function tabSearch(search: string, value: string) {
  const params = new URLSearchParams(search);
  params.set(tabQueryParam, value);
  return `?${params.toString()}`;
}

// 子 Tab 以 URL 为准，仅从当前允许访问的选项中选择。
export function useNavigationTabs<T extends { value: string; label: string }>(tabs: readonly T[]) {
  const { search, hash } = useLocation();
  const navigate = useNavigate();
  const requested = new URLSearchParams(search).get(tabQueryParam);
  const selected = tabs.find(({ value }) => value === requested) ?? tabs[0];
  const value = selected?.value;
  const selectedSearch = value === undefined ? search : tabSearch(search, value);

  useEffect(() => {
    // 缺省、无效或已撤权的 Tab 回退到首个可访问项，不额外增加历史记录。
    if (value === undefined || requested === value) return;
    void navigate({ search: selectedSearch, hash }, { replace: true, preventScrollReset: true });
  }, [hash, navigate, requested, selectedSearch, value]);

  // 链接保留当前空间、对象定位参数和锚点，兼容直接打开及浏览器历史导航。
  const items = tabs.map((tab) => ({
    ...tab,
    to: { search: tabSearch(search, tab.value), hash },
  }));
  return { selected, items };
}
