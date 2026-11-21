import { UserRound, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { useLoadingDelay } from '@/lib/loading';

export function StudentAvatar({
  name,
  src,
  loading = false,
}: {
  name: string;
  src: string;
  loading?: boolean;
}) {
  return <AvatarImage key={src} name={name} src={src} loading={loading} />;
}

function AvatarImage({ name, src, loading }: { name: string; src: string; loading: boolean }) {
  const [status, setStatus] = useState<'loading' | 'loaded' | 'failed'>('loading');
  const pending = useLoadingDelay(loading || (src !== '' && status === 'loading'));
  if (!src || status === 'failed') {
    return (
      <span
        className="glass-panel flex size-20 shrink-0 items-center justify-center rounded-lg border"
        role={pending ? 'status' : undefined}
        aria-label={pending ? '正在加载头像' : `${name}暂无头像`}
        aria-busy={pending}
      >
        {pending ? (
          <LoadingIcon className="size-6 text-muted-foreground" />
        ) : (
          <UserRound className="size-8 text-muted-foreground" aria-hidden="true" />
        )}
      </span>
    );
  }

  const alt = `${name}的头像`;
  return (
    <Dialog.Root>
      <Dialog.Trigger asChild>
        <Button
          type="button"
          variant="ghost"
          aria-label="预览头像"
          aria-busy={pending}
          disabled={pending}
          className="glass-panel relative flex size-20 shrink-0 cursor-zoom-in items-center justify-center rounded-lg border p-0 outline-none hover:bg-transparent focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-100"
        >
          {pending ? (
            <span
              role="status"
              aria-label="正在加载头像"
              className="absolute inset-0 flex items-center justify-center"
            >
              <LoadingIcon className="size-6 text-muted-foreground" />
            </span>
          ) : null}
          <img
            className={`block h-auto max-h-20 w-auto max-w-20 rounded-lg object-contain motion-safe:transition-opacity motion-safe:duration-200 ${pending ? 'opacity-0' : 'opacity-100'}`}
            src={src}
            alt={alt}
            referrerPolicy="no-referrer"
            onLoad={() => {
              setStatus('loaded');
            }}
            onError={() => {
              setStatus('failed');
            }}
          />
        </Button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-[60] bg-overlay/60 backdrop-blur-sm" />
        <Dialog.Content
          aria-describedby={undefined}
          className="glass-panel fixed top-1/2 left-1/2 z-[60] w-max max-w-[92vw] -translate-1/2 rounded-xl border bg-popover shadow-popover outline-none"
        >
          <Dialog.Title className="sr-only">头像预览</Dialog.Title>
          <div className="flex justify-end p-2">
            <Dialog.Close asChild>
              <Button type="button" variant="cancel" size="icon-sm" aria-label="关闭预览">
                <X aria-hidden="true" />
              </Button>
            </Dialog.Close>
          </div>
          <img
            className="block h-auto max-h-[80dvh] w-auto max-w-[90vw] object-contain"
            src={src}
            alt={alt}
            referrerPolicy="no-referrer"
          />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
