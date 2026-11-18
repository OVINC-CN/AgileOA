import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import { MobileNavigation } from '@/components/mobile-navigation';

afterEach(() => {
  vi.unstubAllGlobals();
});

it('全屏菜单支持 Escape 返回入口，恢复桌面宽度时关闭并释放弹层', async () => {
  const listeners = new Set<(event: MediaQueryListEvent) => void>();
  vi.stubGlobal('matchMedia', () => ({
    addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener);
    },
    removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.delete(listener);
    },
  }));
  render(
    <MemoryRouter>
      <MobileNavigation
        workspaceName="虚构空间"
        items={[{ id: 'fixture', label: '虚构页面', to: '/fixture' }]}
      />
    </MemoryRouter>,
  );
  const user = userEvent.setup();
  const trigger = screen.getByRole('button', { name: '打开导航菜单' });
  expect(trigger).toHaveTextContent('菜单');
  await user.click(trigger);
  expect(screen.getByRole('dialog', { name: '导航菜单' })).toBeVisible();
  await user.keyboard('{Escape}');
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: '导航菜单' })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  await user.click(trigger);
  act(() => {
    listeners.forEach((listener) => {
      listener({ matches: true } as MediaQueryListEvent);
    });
  });
  expect(screen.queryByRole('dialog', { name: '导航菜单' })).not.toBeInTheDocument();
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
});
