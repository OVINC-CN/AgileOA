import { CircleAlert, CircleCheck, Info, TriangleAlert, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { createPortal } from 'react-dom';

import { Button } from '@/components/ui/button';
import { dismissMessage, messageStore, type MessageTone, showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

type Message = ReturnType<typeof messageStore.getSnapshot>['messages'][number];

const toneStyles: Record<
  MessageTone,
  {
    role: 'alert' | 'status';
    border: string;
    icon: typeof CircleAlert;
    iconClassName: string;
  }
> = {
  error: {
    role: 'alert',
    border: 'border-danger-border',
    icon: CircleAlert,
    iconClassName: 'text-danger-foreground',
  },
  warning: {
    role: 'alert',
    border: 'border-warning-border',
    icon: TriangleAlert,
    iconClassName: 'text-warning-foreground',
  },
  success: {
    role: 'status',
    border: 'border-success-border',
    icon: CircleCheck,
    iconClassName: 'text-success-foreground',
  },
  info: {
    role: 'status',
    border: 'border-info-border',
    icon: Info,
    iconClassName: 'text-info-foreground',
  },
};

function MessageItem({
  message,
  present,
  onExitComplete,
}: {
  message: Message;
  present: boolean;
  onExitComplete: (id: string) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // 关闭后保留消息直到退出动画结束，无动画时立即清理。
    const element = ref.current;
    if (present || !element) return;
    const animationName = getComputedStyle(element).animationName;
    if (!animationName || animationName === 'none') {
      onExitComplete(message.id);
      return;
    }
    let cancelled = false;
    void Promise.allSettled(element.getAnimations().map((animation) => animation.finished)).then(
      () => {
        if (!cancelled) onExitComplete(message.id);
      },
    );

    // 消息重新出现或挂载位置改变后，忽略旧动画的结束回调。
    return () => {
      cancelled = true;
    };
  }, [message.id, present, onExitComplete]);
  const tone = toneStyles[message.tone];
  const Icon = tone.icon;
  return (
    <div
      ref={ref}
      id={message.id}
      role={tone.role}
      data-tone={message.tone}
      aria-hidden={!present}
      inert={!present}
      data-state={present ? 'open' : 'closed'}
      style={{ pointerEvents: present ? 'auto' : 'none' }}
      className={cn(
        'message-motion glass-panel flex w-full shrink-0 items-start gap-2.5 rounded-xl border bg-popover px-3.5 py-2.5 text-sm shadow-popover',
        tone.border,
      )}
    >
      <Icon aria-hidden="true" className={cn('mt-0.5 size-4 shrink-0', tone.iconClassName)} />
      <p className="min-w-0 flex-1 py-0.5 leading-5 wrap-anywhere whitespace-pre-wrap">
        {message.text}
      </p>
      {message.action ? (
        <Button type="button" size="xs" onClick={message.action.onClick}>
          {message.action.label}
        </Button>
      ) : null}
      {message.dismissible ? (
        <Button
          type="button"
          variant="cancel"
          size="icon-xs"
          className="border-0 text-muted-foreground enabled:hover:text-foreground"
          aria-label="关闭消息"
          onClick={() => {
            dismissMessage(message.id);
          }}
        >
          <X aria-hidden="true" />
        </Button>
      ) : null}
    </div>
  );
}

export function MessageViewport() {
  const { messages, container } = useSyncExternalStore(
    messageStore.subscribe,
    messageStore.getSnapshot,
  );
  const [rendered, setRendered] = useState({ source: messages, messages });

  // 保留退出中的消息及其原位置，仍按消息源更新有效内容和顺序。
  if (rendered.source !== messages) {
    const retained = [...messages];
    rendered.messages.forEach((message, index) => {
      if (!messages.some((item) => item.id === message.id)) retained.splice(index, 0, message);
    });
    setRendered({ source: messages, messages: retained });
  }
  const onExitComplete = useCallback((id: string) => {
    // 同编号消息可能已重新出现，旧动画不得清理新消息。
    if (messageStore.getSnapshot().messages.some((message) => message.id === id)) return;
    setRendered((current) => ({
      ...current,
      messages: current.messages.filter((message) => message.id !== id),
    }));
  }, []);

  useEffect(() => {
    const invalid = (event: Event) => {
      const field = event.target;
      if (!(
        field instanceof HTMLInputElement ||
        field instanceof HTMLSelectElement ||
        field instanceof HTMLTextAreaElement
      ))
        return;
      event.preventDefault();
      const label =
        field.labels?.[0]?.textContent.trim() ?? field.getAttribute('aria-label') ?? '表单内容';
      const reason = field.validity.valueMissing
        ? '请填写此项'
        : field.validity.tooLong
          ? '内容超过长度限制'
          : field.validity.tooShort
            ? '内容长度不足'
            : '内容格式不正确，请检查';
      showMessage(`${label}：${reason}`);
      if (field.form?.querySelector('input:invalid, textarea:invalid, select:invalid') === field)
        field.focus();
    };
    document.addEventListener('invalid', invalid, true);
    return () => {
      document.removeEventListener('invalid', invalid, true);
    };
  }, []);
  return createPortal(
    <section
      aria-label="消息通知"
      aria-live="polite"
      aria-relevant="additions"
      style={{ pointerEvents: 'none' }}
      className="fixed inset-x-4 top-4 z-[100] mx-auto flex max-h-[40svh] max-w-sm flex-col gap-2 overflow-y-auto"
    >
      {rendered.messages.map((message) => (
        <MessageItem
          key={message.id}
          message={message}
          present={messages.some((item) => item.id === message.id)}
          onExitComplete={onExitComplete}
        />
      ))}
    </section>,
    container ?? document.body,
  );
}
