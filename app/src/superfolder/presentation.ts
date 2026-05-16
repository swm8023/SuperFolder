import { DirectoryEntry } from './types';

export type EntryIconKind = 'folder' | 'file';

export interface EntryPresentation {
  icon: EntryIconKind;
  kindLabel: string;
  badges: string[];
}

const formatterCache = new Map<string, Intl.DateTimeFormat>();

export function entryPresentation(entry: DirectoryEntry): EntryPresentation {
  return {
    icon: entry.kind === 'directory' ? 'folder' : 'file',
    kindLabel: entry.kind === 'directory' ? 'Folder' : fileKindLabel(entry.name),
    badges: entryBadges(entry),
  };
}

export function formatEntrySize(entry: DirectoryEntry): string {
  if (entry.kind === 'directory') return '--';
  if (entry.size < 1024) return `${entry.size} B`;

  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = entry.size / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${formatNumber(value)} ${units[unitIndex]}`;
}

export function formatEntryTime(entry: DirectoryEntry, locale = navigator.language): string {
  const formatter = getFormatter(locale);
  return formatter.format(new Date(entry.mtime));
}

function fileKindLabel(name: string): string {
  const dot = name.lastIndexOf('.');
  if (dot <= 0 || dot === name.length - 1) return 'File';
  return `${name.slice(dot + 1).toUpperCase()} File`;
}

function entryBadges(entry: DirectoryEntry): string[] {
  const badges: string[] = [];
  if (entry.hidden) badges.push('Hidden');
  if (entry.readonly) badges.push('Read-only');
  if (entry.system) badges.push('System');
  return badges;
}

function formatNumber(value: number): string {
  return value >= 10 ? value.toFixed(0) : value.toFixed(1);
}

function getFormatter(locale: string): Intl.DateTimeFormat {
  const cached = formatterCache.get(locale);
  if (cached) return cached;
  const formatter = new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
  formatterCache.set(locale, formatter);
  return formatter;
}
