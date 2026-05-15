export type ViewMode = 'details' | 'tiles' | 'tree';
export type SortKey = 'name' | 'kind' | 'size' | 'mtime';
export type SortDirection = 'asc' | 'desc';
export type EntryKind = 'file' | 'directory';
export type ClipboardMode = 'copy' | 'cut';
export type JobStatus = 'queued' | 'running' | 'waiting_conflict' | 'cancelling' | 'completed' | 'failed' | 'cancelled';
export type PreviewKind = 'text' | 'image';

export interface SessionState {
  version: number;
  windows: WorkspaceWindowState[];
}

export interface WorkspaceWindowState {
  id: string;
  panes: PaneState[];
  activePaneId: string;
  utilityPanel: UtilityPanelState;
}

export interface PaneState {
  id: string;
  tabs: BrowserTabState[];
  activeTabId: string;
}

export interface BrowserTabState {
  id: string;
  title: string;
  path: string;
  viewMode: ViewMode;
  sortKey: SortKey;
  sortDirection: SortDirection;
  filterText: string;
  expandedPaths: string[];
}

export interface UtilityPanelState {
  collapsed: boolean;
  height: number;
  activeTab: string;
}

export interface FavoriteItem {
  id: string;
  name: string;
  path: string;
  kind: 'folder';
}

export interface DirectoryEntry {
  name: string;
  path: string;
  kind: EntryKind;
  size: number;
  mtime: number;
  readonly: boolean;
  hidden: boolean;
  system: boolean;
  hasChildren: boolean;
}

export interface ListChildrenRequest {
  path: string;
  knownHash?: string;
  viewMode: ViewMode;
  sortKey: SortKey;
  sortDirection: SortDirection;
  filterText: string;
}

export interface ListChildrenResponse {
  path: string;
  unchanged: boolean;
  childrenHash: string;
  entries?: DirectoryEntry[];
}

export interface ClipboardState {
  mode: ClipboardMode;
  paths: string[];
  sourcePaneId: string;
  sourceTabId: string;
}

export interface JobSnapshot {
  id: string;
  kind: string;
  status: JobStatus;
  sources: string[];
  targetDir: string;
  newName: string;
  total: number;
  completed: number;
  skipped: number;
  conflict?: { source: string; target: string };
  error?: { code: number; message: string };
}

export interface GitSummary {
  path: string;
  isRepo: boolean;
  repoRoot: string;
  branch: string;
  changed: number;
  logs: Array<{ hash: string; subject: string }>;
  error?: string;
}

export interface PreviewResponse {
  path: string;
  kind: PreviewKind;
  mime: string;
  text?: string;
  dataUrl?: string;
  truncated: boolean;
}
