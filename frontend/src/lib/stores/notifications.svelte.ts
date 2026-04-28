export type NotificationType = 'success' | 'error' | 'warning' | 'info';

export type Notification = {
  id: number;
  type: NotificationType;
  title: string;
  message?: string;
  duration: number;
};

function createNotificationStore() {
  let items = $state<Notification[]>([]);
  let nextId = 0;
  const timers = new Map<number, ReturnType<typeof setTimeout>>();

  function push(type: NotificationType, title: string, message?: string, duration = 3000) {
    const id = ++nextId;
    items.push({ id, type, title, message, duration });
    if (duration > 0) {
      timers.set(id, setTimeout(() => dismiss(id), duration));
    }
    return id;
  }

  function dismiss(id: number) {
    const timer = timers.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.delete(id);
    }
    const idx = items.findIndex((n) => n.id === id);
    if (idx !== -1) items.splice(idx, 1);
  }

  return {
    get items() { return items; },
    dismiss,
    success: (title: string, message?: string, duration?: number) => push('success', title, message, duration),
    error:   (title: string, message?: string, duration?: number) => push('error',   title, message, duration),
    warning: (title: string, message?: string, duration?: number) => push('warning', title, message, duration),
    info:    (title: string, message?: string, duration?: number) => push('info',    title, message, duration)
  };
}

export const notify = createNotificationStore();
