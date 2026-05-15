export type ShortcutCommand =
  | 'open'
  | 'rename'
  | 'delete'
  | 'deletePermanent'
  | 'copy'
  | 'cut'
  | 'paste'
  | 'focusPath'
  | 'newTab'
  | 'closeTab'
  | 'historyBack'
  | 'historyForward'
  | 'up';

export interface KeyboardLike {
  key: string;
  ctrlKey?: boolean;
  shiftKey?: boolean;
  altKey?: boolean;
}

export function mapKeyboardShortcut(event: KeyboardLike): ShortcutCommand | null {
  const key = event.key.toLowerCase();
  if (event.altKey && event.key === 'ArrowLeft') return 'historyBack';
  if (event.altKey && event.key === 'ArrowRight') return 'historyForward';
  if (event.ctrlKey && key === 'c') return 'copy';
  if (event.ctrlKey && key === 'x') return 'cut';
  if (event.ctrlKey && key === 'v') return 'paste';
  if (event.ctrlKey && key === 'l') return 'focusPath';
  if (event.ctrlKey && key === 't') return 'newTab';
  if (event.ctrlKey && key === 'w') return 'closeTab';
  if (event.key === 'Enter') return 'open';
  if (event.key === 'F2') return 'rename';
  if (event.key === 'Delete' && event.shiftKey) return 'deletePermanent';
  if (event.key === 'Delete') return 'delete';
  if (event.key === 'Backspace') return 'up';
  return null;
}
