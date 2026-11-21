import { CircleAlert, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { createPortal } from 'react-dom';

import { Button } from '@/components/ui/button';
import { dismissMessage, messageStore, showMessage } from '@/lib/messages';

type Message = ReturnType<typeof messageStore.getSnapshot>['messages'][number];

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
  return (
    <div
      ref={ref}
      id={message.id}
      role="alert"
      aria-hidden={!present}
      inert={!present}
      data-state={present ? 'open' : 'closed'}
      style={{ pointerEvents: present ? 'auto' : 'none' }}
      className="message-motion glass-panel flex shrink-0 items-start gap-3 rounded-xl border border-destructive/25 bg-popover px-4 py-3 text-sm shadow-none"
    >
      <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-destructive" />
      <p className="min-w-0 flex-1 py-0.5 break-words">{message.text}</p>
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
      aria-live="assertive"
      aria-relevant="additions"
      style={{ pointerEvents: 'none' }}
      className="fixed inset-x-4 top-4 z-[100] mx-auto flex max-h-[40svh] max-w-lg flex-col gap-2 overflow-y-auto"
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
