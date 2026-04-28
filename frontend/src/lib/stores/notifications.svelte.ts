export type NotificationType = 'success' | 'error' | 'warning' | 'info';

export type Notification = {
  id: number;
  type: NotificationType;
  title: string;
  message?: string;
  duration: number;
  link?: string;
};

function createNotificationStore() {
  let items = $state<Notification[]>([]);
  let nextId = 0;
  const timers = new Map<number, ReturnType<typeof setTimeout>>();

  function push(
    type: NotificationType,
    title: string,
    message?: string,
    duration = 3000,
    link?: string
  ) {
    const id = ++nextId;
    items.push({ id, type, title, message, duration, link });
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
    success: (title: string, message?: string, duration?: number, link?: string) => push('success', title, message, duration, link),
    error:   (title: string, message?: string, duration?: number, link?: string) => push('error',   title, message, duration, link),
    warning: (title: string, message?: string, duration?: number, link?: string) => push('warning', title, message, duration, link),
    info:    (title: string, message?: string, duration?: number, link?: string) => push('info',    title, message, duration, link)
  };
}

export const notify = createNotificationStore();

type FormResult =
  | { type: 'success'; status: number; data?: Record<string, unknown> }
  | { type: 'failure'; status: number; data?: Record<string, unknown> }
  | { type: 'redirect'; status: number; location: string }
  | { type: 'error'; status?: number; error: { message?: string } };

export function showResult(result: FormResult, successTitle: string, errorTitle = 'Operation failed') {
  if (result.type === 'success' || result.type === 'redirect') {
    notify.success(successTitle);
  } else if (result.type === 'failure') {
    const err = result.data?.error;
    notify.error(errorTitle, typeof err === 'string' ? err : undefined);
  } else if (result.type === 'error') {
    notify.error(errorTitle, result.error?.message ?? 'Please try again.');
  }
}
