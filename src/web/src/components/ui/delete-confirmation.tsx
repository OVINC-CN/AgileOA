import { type LucideIcon, Trash2 } from 'lucide-react';
import { type ComponentProps, useEffect } from 'react';

import { Button } from '@/components/ui/button';
import { type DeleteConfirmationAction, useDeleteConfirmation } from '@/lib/delete-confirmation';

export function DeleteConfirmation({
  target,
  onConfirm,
  blocked,
  disabled,
  type = 'button',
  size = 'sm',
  variant = 'table-action',
  label = '删除',
  icon: Icon = Trash2,
  'aria-label': ariaLabel,
}: DeleteConfirmationAction &
  Pick<ComponentProps<typeof Button>, 'disabled' | 'type' | 'size' | 'variant' | 'aria-label'> & {
    label?: string;
    icon?: LucideIcon;
  }) {
  const { target: selected, pending, select, cancel } = useDeleteConfirmation();
  const confirming = selected === target;
  useEffect(() => {
    if (!confirming) return;
    const onEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      // 先取消按钮的确认状态，再由下一次 Escape 处理外围抽屉。
      event.preventDefault();
      event.stopPropagation();
      cancel();
    };
    window.addEventListener('keydown', onEscape, true);
    return () => {
      window.removeEventListener('keydown', onEscape, true);
    };
  }, [confirming, cancel]);

  return (
    <Button
      type={type}
      size={size}
      variant={confirming ? 'default' : variant}
      className="motion-safe:duration-(--motion-delete-confirmation-duration)"
      disabled={disabled}
      loading={pending}
      aria-label={confirming ? undefined : ariaLabel}
      aria-description={confirming ? `确认删除 ${target} ?` : undefined}
      onBlur={cancel}
      onKeyDown={(event) => {
        if (event.repeat && (event.key === 'Enter' || event.key === ' ')) event.preventDefault();
      }}
      onClick={(event) => {
        event.preventDefault();
        if (event.detail > 1) return;
        if (!confirming) event.currentTarget.focus({ preventScroll: true });
        select({ target, onConfirm, blocked });
      }}
    >
      <Icon aria-hidden="true" />
      <span
        key={String(confirming)}
        className={
          confirming
            ? 'motion-safe:animate-in motion-safe:duration-(--motion-delete-confirmation-duration) motion-safe:fade-in'
            : undefined
        }
      >
        {confirming ? '确认' : label}
      </span>
    </Button>
  );
}
