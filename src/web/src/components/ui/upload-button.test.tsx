import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { Drawer } from '@/components/ui/drawer';
import { UploadButton } from '@/components/ui/upload-button';
import type { UploadStatus } from '@/lib/file-upload';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('上传进度浮层自动关闭', () => {
  it('抽屉关闭进入退场态时立即隐藏并关闭已完成的上传进度', async () => {
    const onDismiss = vi.fn();
    let finishPanel: (() => void) | undefined;
    const panelFinished = new Promise<void>((resolve) => {
      finishPanel = resolve;
    });

    function Example() {
      const [progress, setProgress] = useState<UploadStatus>('complete');
      return (
        <Drawer title="附件" onClose={vi.fn()}>
          <UploadButton
            progress={progress}
            onDismiss={() => {
              onDismiss();
              setProgress(null);
            }}
          >
            上传
          </UploadButton>
        </Drawer>
      );
    }

    render(<Example />);
    expect(screen.getByRole('group', { name: '文件上传状态' })).toBeVisible();
    expect(screen.getByRole('button', { name: '关闭上传进度' })).toHaveClass('border-0');
    const dialog = screen.getByRole('dialog', { name: '附件' });
    const panel = dialog.querySelector('.drawer-panel');
    if (!panel) throw new Error('缺少抽屉面板');
    Object.defineProperty(panel, 'getAnimations', {
      value: () => [{ finished: panelFinished }],
    });

    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    await waitFor(() => {
      expect(onDismiss).toHaveBeenCalledOnce();
      expect(screen.queryByRole('group', { name: '文件上传状态' })).not.toBeInTheDocument();
    });
    await act(() => {
      finishPanel?.();
      return panelFinished;
    });
  });

  it('上传按钮滚出可视区时隐藏浮层，完成后只关闭一次', async () => {
    let notifyIntersection: ((entries: IntersectionObserverEntry[]) => void) | undefined;
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(callback: (entries: IntersectionObserverEntry[]) => void) {
          notifyIntersection = callback;
        }
        observe = vi.fn();
        unobserve = vi.fn();
        disconnect = vi.fn();
      },
    );

    const onDismiss = vi.fn();
    function Example({ loading }: { loading: boolean }) {
      const [dismissed, setDismissed] = useState(false);
      return (
        <UploadButton
          loading={loading}
          progress={loading ? 50 : dismissed ? null : 'complete'}
          onDismiss={() => {
            onDismiss();
            setDismissed(true);
          }}
        >
          上传
        </UploadButton>
      );
    }

    const { rerender } = render(<Example loading />);
    expect(screen.getByRole('group', { name: '文件上传状态' })).toBeVisible();
    act(() => {
      notifyIntersection?.([{ isIntersecting: false } as IntersectionObserverEntry]);
    });
    await waitFor(() => {
      expect(screen.queryByRole('group', { name: '文件上传状态' })).not.toBeInTheDocument();
    });
    expect(onDismiss).not.toHaveBeenCalled();

    rerender(<Example loading={false} />);
    expect(onDismiss).toHaveBeenCalledOnce();
    expect(screen.queryByRole('group', { name: '文件上传状态' })).not.toBeInTheDocument();
  });
});
