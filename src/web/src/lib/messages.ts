import { getConfig } from '@/lib/config';

interface MessageOptions {
  // 设为 0 时常驻，普通消息默认按运行配置自动关闭。
  durationMs?: number;
  dismissible?: boolean;
  action?: {
    label: string;
    onClick: () => void;
  };
}
interface Message {
  id: string;
  text: string;
  dismissible: boolean;
  action?: MessageOptions['action'];
}
interface Snapshot {
  messages: Message[];
  container: HTMLElement | null;
}
let snapshot: Snapshot = { messages: [], container: null };
let nextID = 0;
const listeners = new Set<() => void>();
const timers = new Map<string, ReturnType<typeof setTimeout>>();
const publish = (next: Snapshot) => {
  snapshot = next;
  listeners.forEach((listener) => {
    listener();
  });
};
export const messageStore = {
  subscribe: (listener: () => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
  getSnapshot: () => snapshot,
};
export function showMessage(text: string, id?: string, options: MessageOptions = {}) {
  const content = text.trim();
  if (!content || snapshot.messages.some((message) => message.text === content)) return;
  const messageID = id ?? `message-${String(++nextID)}`;
  // 同一编号替换内容时重新计时，独立消息互不影响。
  clearTimeout(timers.get(messageID));
  timers.delete(messageID);
  const durationMs = options.durationMs ?? getConfig().messageDurationMs;
  if (durationMs > 0) {
    timers.set(
      messageID,
      setTimeout(() => {
        dismissMessage(messageID);
      }, durationMs),
    );
  }
  publish({
    ...snapshot,
    messages: [
      ...snapshot.messages.filter((message) => message.id !== messageID),
      {
        id: messageID,
        text: content,
        dismissible: options.dismissible ?? true,
        action: options.action,
      },
    ],
  });
}
export function dismissMessage(id: string) {
  clearTimeout(timers.get(id));
  timers.delete(id);
  publish({ ...snapshot, messages: snapshot.messages.filter((message) => message.id !== id) });
}
export function clearMessages() {
  timers.forEach((timer) => {
    clearTimeout(timer);
  });
  timers.clear();
  publish({ ...snapshot, messages: [] });
}
// 消息始终挂载到最上层抽屉的焦点范围，关闭嵌套抽屉后恢复到上一层。
const containers: HTMLElement[] = [];
export function setMessageContainer(container: HTMLElement | null) {
  if (!container) return;
  containers.push(container);
  updateMessageContainer();
  return () => {
    const index = containers.lastIndexOf(container);
    if (index >= 0) containers.splice(index, 1);
    updateMessageContainer();
  };
}

function updateMessageContainer() {
  // 按实际叠放的 DOM 顺序选择最上层，不依赖嵌套组件的 ref 提交先后。
  const container = containers.reduce<HTMLElement | null>((active, element) => {
    if (!element.isConnected) return active;
    if (!active || active.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING)
      return element;
    return active;
  }, null);
  if (container !== snapshot.container) publish({ ...snapshot, container });
}
