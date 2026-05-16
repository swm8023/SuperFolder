import { useVirtualizer } from '@tanstack/react-virtual';
import {
  ChevronRight,
  File as FileIcon,
  Folder as FolderIcon,
  Grid2X2,
  List,
  ListTree,
  Plus,
  RefreshCw,
  X,
  type LucideIcon,
} from 'lucide-react';
import { CSSProperties, MouseEvent, useRef } from 'react';
import { entryPresentation, formatEntrySize, formatEntryTime } from './presentation';
import { DirectoryEntry, FavoriteItem, GitSummary, JobSnapshot, ListChildrenRequest, PaneState, PreviewResponse, SessionState, ViewMode } from './types';

export function Sidebar({ favorites, onOpen }: { favorites: FavoriteItem[]; onOpen: (path: string) => void }) {
  return (
    <aside className="sf-sidebar">
      <div className="sf-sidebar-title">收藏目录</div>
      <div className="sf-favorites">
        {favorites.map((favorite) => (
          <button key={favorite.id} type="button" className="sf-favorite" onClick={() => onOpen(favorite.path)} title={favorite.path}>
            <span className="sf-entry-icon folder compact">
              <FolderIcon size={15} />
            </span>
            <span className="sf-favorite-label">{favorite.name}</span>
            <span className="sf-favorite-path">{favorite.path}</span>
          </button>
        ))}
      </div>
    </aside>
  );
}

export interface BrowserPaneProps {
  pane: PaneState;
  entries: DirectoryEntry[];
  loading: boolean;
  selectedPaths: string[];
  editingPath: string;
  editName: string;
  onEditNameChange: (name: string) => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  onSelect: (entry: DirectoryEntry, event: { ctrlKey?: boolean; shiftKey?: boolean }) => void;
  onOpen: (entry: DirectoryEntry) => void;
  onPathSubmit: (path: string) => void;
  onViewModeChange: (viewMode: ViewMode) => void;
  onRefresh: () => void;
  onNewTab: () => void;
  onCloseTab: (tabId: string) => void;
  onActivateTab: (tabId: string) => void;
  onContextMenu: (event: MouseEvent, entry?: DirectoryEntry) => void;
}

export function BrowserPane(props: BrowserPaneProps) {
  const activeTab = props.pane.tabs.find((tab) => tab.id === props.pane.activeTabId) ?? props.pane.tabs[0];
  if (!activeTab) {
    return <section className="sf-pane" />;
  }

  return (
    <section className="sf-pane" onContextMenu={(event) => props.onContextMenu(event)}>
      <div className="sf-tabs">
        {props.pane.tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className={tab.id === activeTab.id ? 'sf-tab active' : 'sf-tab'}
            onClick={() => props.onActivateTab(tab.id)}
            title={tab.path}
          >
            {tab.title || tab.path}
            {props.pane.tabs.length > 1 ? (
              <span
                className="sf-tab-close"
                title="Close tab"
                onClick={(event) => {
                  event.stopPropagation();
                  props.onCloseTab(tab.id);
                }}
              >
                <X size={13} />
              </span>
            ) : null}
          </button>
        ))}
        <button type="button" className="sf-tab add" onClick={props.onNewTab} title="New tab">
          <Plus size={15} />
        </button>
      </div>

      <form
        className="sf-pathbar"
        onSubmit={(event) => {
          event.preventDefault();
          const form = event.currentTarget;
          const input = form.elements.namedItem('path') as HTMLInputElement;
          props.onPathSubmit(input.value);
        }}
      >
        <input name="path" defaultValue={activeTab.path} key={activeTab.path} spellCheck={false} />
        <button type="submit">Go</button>
        <button type="button" onClick={props.onRefresh}>
          <RefreshCw size={14} />
          Refresh
        </button>
      </form>

      <div className="sf-pane-toolbar">
        <ViewButton label="Details" icon={List} mode="details" active={activeTab.viewMode === 'details'} onClick={props.onViewModeChange} />
        <ViewButton label="Tiles" icon={Grid2X2} mode="tiles" active={activeTab.viewMode === 'tiles'} onClick={props.onViewModeChange} />
        <ViewButton label="Tree" icon={ListTree} mode="tree" active={activeTab.viewMode === 'tree'} onClick={props.onViewModeChange} />
        {props.loading ? <span className="sf-muted">Loading</span> : null}
      </div>

      {activeTab.viewMode === 'tiles' ? <TilesView {...props} /> : activeTab.viewMode === 'tree' ? <TreeView {...props} /> : <DetailsView {...props} />}
    </section>
  );
}

function ViewButton({
  label,
  icon: Icon,
  mode,
  active,
  onClick,
}: {
  label: string;
  icon: LucideIcon;
  mode: ViewMode;
  active: boolean;
  onClick: (mode: ViewMode) => void;
}) {
  return (
    <button type="button" className={active ? 'sf-tool active' : 'sf-tool'} onClick={() => onClick(mode)} title={label}>
      <Icon size={15} />
      {label}
    </button>
  );
}

function DetailsView(props: BrowserPaneProps) {
  const parentRef = useRef<HTMLDivElement | null>(null);
  const rowVirtualizer = useVirtualizer({
    count: props.entries.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 32,
    overscan: 16,
  });
  const virtualRows = rowVirtualizer.getVirtualItems();

  return (
    <div className="sf-details" role="grid" ref={parentRef}>
      <div className="sf-row header">
        <span>Name</span>
        <span>Kind</span>
        <span>Size</span>
        <span>Modified</span>
      </div>
      {props.entries.length === 0 ? <div className="sf-empty sf-list-empty">This folder is empty.</div> : null}
      <div className="sf-virtual-body" style={{ height: rowVirtualizer.getTotalSize() }}>
        {virtualRows.map((virtualRow) => {
          const entry = props.entries[virtualRow.index];
          return (
            <EntryRow
              key={entry.path}
              entry={entry}
              rowStyle={{
                height: virtualRow.size,
                transform: `translateY(${virtualRow.start}px)`,
              }}
              {...props}
            />
          );
        })}
      </div>
    </div>
  );
}

function EntryRow(props: BrowserPaneProps & { entry: DirectoryEntry; rowStyle?: CSSProperties }) {
  const selected = props.selectedPaths.includes(props.entry.path);
  const presentation = entryPresentation(props.entry);
  return (
    <div
      className={selected ? 'sf-row selected' : 'sf-row'}
      style={props.rowStyle}
      onClick={(event) => props.onSelect(props.entry, event)}
      onDoubleClick={() => props.onOpen(props.entry)}
      onContextMenu={(event) => props.onContextMenu(event, props.entry)}
      role="row"
    >
      <span className="sf-name-cell">
        <EntryGlyph entry={props.entry} />
        {props.editingPath === props.entry.path ? (
          <input
            className="sf-inline-edit"
            value={props.editName}
            autoFocus
            onChange={(event) => props.onEditNameChange(event.target.value)}
            onBlur={props.onCommitRename}
            onKeyDown={(event) => {
              if (event.key === 'Enter') props.onCommitRename();
              if (event.key === 'Escape') props.onCancelRename();
            }}
          />
        ) : (
          <>
            <span className="sf-entry-name" title={props.entry.path}>
              {props.entry.name}
            </span>
            {presentation.badges.map((badge) => (
              <span key={badge} className="sf-entry-badge">
                {badge}
              </span>
            ))}
          </>
        )}
      </span>
      <span className="sf-kind-cell">{presentation.kindLabel}</span>
      <span className="sf-size-cell">{formatEntrySize(props.entry)}</span>
      <span className="sf-time-cell">{formatEntryTime(props.entry)}</span>
    </div>
  );
}

function TilesView(props: BrowserPaneProps) {
  return (
    <div className="sf-tiles">
      {props.entries.map((entry) => (
        <button
          key={entry.path}
          type="button"
          className={props.selectedPaths.includes(entry.path) ? 'sf-tile selected' : 'sf-tile'}
          onClick={(event) => props.onSelect(entry, event)}
          onDoubleClick={() => props.onOpen(entry)}
          onContextMenu={(event) => props.onContextMenu(event, entry)}
          title={entry.path}
        >
          <EntryGlyph entry={entry} size="large" />
          <span className="sf-tile-name">{entry.name}</span>
          <span className="sf-tile-meta">{entryPresentation(entry).kindLabel}</span>
        </button>
      ))}
    </div>
  );
}

function TreeView(props: BrowserPaneProps) {
  return (
    <div className="sf-tree-view">
      {props.entries.map((entry) => (
        <button
          key={entry.path}
          type="button"
          className={props.selectedPaths.includes(entry.path) ? 'sf-tree-node selected' : 'sf-tree-node'}
          onClick={(event) => props.onSelect(entry, event)}
          onDoubleClick={() => props.onOpen(entry)}
          onContextMenu={(event) => props.onContextMenu(event, entry)}
        >
          <span className="sf-tree-expander">{entry.kind === 'directory' ? <ChevronRight size={14} /> : null}</span>
          <EntryGlyph entry={entry} />
          <span className="sf-tree-name">{entry.name}</span>
          <span className="sf-tree-meta">{entryPresentation(entry).kindLabel}</span>
        </button>
      ))}
    </div>
  );
}

function EntryGlyph({ entry, size = 'normal' }: { entry: DirectoryEntry; size?: 'compact' | 'normal' | 'large' }) {
  const presentation = entryPresentation(entry);
  const Icon = presentation.icon === 'folder' ? FolderIcon : FileIcon;
  return (
    <span className={`sf-entry-icon ${presentation.icon} ${size}`}>
      <Icon size={size === 'large' ? 28 : 16} />
    </span>
  );
}

export function UtilityPanel({
  session,
  gitSummary,
  preview,
  selectedPath,
  onTabChange,
  onHeightChange,
  onToggleCollapsed,
  onRefreshGit,
}: {
  session: SessionState;
  gitSummary: GitSummary | null;
  preview: PreviewResponse | null;
  selectedPath: string;
  onTabChange: (tab: string) => void;
  onHeightChange: (height: number) => void;
  onToggleCollapsed: () => void;
  onRefreshGit: () => void;
}) {
  const panel = session.windows[0]?.utilityPanel;
  if (!panel) return null;
  return (
    <section className={panel.collapsed ? 'sf-utility collapsed' : 'sf-utility'} style={{ height: panel.collapsed ? 42 : panel.height }}>
      <div className="sf-utility-tabs">
        {['terminal', 'git', 'p4', 'preview'].map((tab) => (
          <button key={tab} type="button" className={panel.activeTab === tab ? 'active' : ''} onClick={() => onTabChange(tab)}>
            {tab}
          </button>
        ))}
        <button type="button" onClick={onToggleCollapsed}>
          {panel.collapsed ? 'Expand' : 'Collapse'}
        </button>
        {!panel.collapsed ? (
          <input type="range" min={160} max={420} value={panel.height} onChange={(event) => onHeightChange(Number(event.target.value))} />
        ) : null}
      </div>
      {!panel.collapsed ? (
        <div className="sf-utility-body">
          {panel.activeTab === 'terminal' ? <div className="sf-empty">Terminal will be enabled in a later slice.</div> : null}
          {panel.activeTab === 'p4' ? <div className="sf-empty">P4 will be enabled in a later slice.</div> : null}
          {panel.activeTab === 'git' ? <GitPanel summary={gitSummary} onRefresh={onRefreshGit} /> : null}
          {panel.activeTab === 'preview' ? <PreviewPanel preview={preview} selectedPath={selectedPath} /> : null}
        </div>
      ) : null}
    </section>
  );
}

function GitPanel({ summary, onRefresh }: { summary: GitSummary | null; onRefresh: () => void }) {
  return (
    <div className="sf-git-panel">
      <button type="button" onClick={onRefresh}>
        Refresh Git
      </button>
      {!summary || !summary.isRepo ? (
        <div className="sf-empty">No Git repository detected.</div>
      ) : (
        <div>
          <div className="sf-kv-line">Root: {summary.repoRoot}</div>
          <div className="sf-kv-line">Branch: {summary.branch || '-'}</div>
          <div className="sf-kv-line">Changed: {summary.changed}</div>
          <ol>
            {summary.logs.map((log) => (
              <li key={`${log.hash}-${log.subject}`}>
                <code>{log.hash}</code> {log.subject}
              </li>
            ))}
          </ol>
        </div>
      )}
    </div>
  );
}

function PreviewPanel({ preview, selectedPath }: { preview: PreviewResponse | null; selectedPath: string }) {
  if (!selectedPath) return <div className="sf-empty">Select a file to preview.</div>;
  if (!preview) return <div className="sf-empty">No preview loaded.</div>;
  if (preview.kind === 'image') return <img className="sf-preview-image" src={preview.dataUrl} alt={preview.path} />;
  return <pre className="sf-preview-text">{preview.text}</pre>;
}

export function JobQueue({ jobs, onCancel }: { jobs: JobSnapshot[]; onCancel: (jobId: string) => void }) {
  return (
    <aside className="sf-jobs">
      <div className="sf-sidebar-title">后台任务</div>
      {jobs.length === 0 ? (
        <div className="sf-empty">暂无后台任务</div>
      ) : (
        jobs.map((job) => (
          <div key={job.id} className="sf-job">
            <div>
              <strong>{job.kind}</strong> {job.status}
            </div>
            <div className="sf-muted">
              {job.completed}/{job.total}
              {job.skipped ? ` skipped ${job.skipped}` : ''}
            </div>
            {job.error ? <div className="sf-error">{job.error.message}</div> : null}
            {job.status === 'queued' || job.status === 'running' || job.status === 'waiting_conflict' ? (
              <button type="button" onClick={() => onCancel(job.id)}>
                取消
              </button>
            ) : null}
          </div>
        ))
      )}
    </aside>
  );
}

export function ConflictDialog({
  job,
  onResolve,
}: {
  job: JobSnapshot | null;
  onResolve: (jobId: string, action: 'overwrite' | 'skip' | 'keep_both', applyToAll: boolean) => void;
}) {
  if (!job?.conflict) return null;
  return (
    <div className="sf-modal-backdrop">
      <div className="sf-modal">
        <h2>Name conflict</h2>
        <p>{job.conflict.target}</p>
        <label>
          <input id="apply-conflict-to-all" type="checkbox" /> Apply to all
        </label>
        <div className="sf-modal-actions">
          {[
            ['overwrite', 'Overwrite'],
            ['skip', 'Skip'],
            ['keep_both', 'Keep Both'],
          ].map(([action, label]) => (
            <button
              key={action}
              type="button"
              onClick={() => {
                const input = document.getElementById('apply-conflict-to-all') as HTMLInputElement | null;
                onResolve(job.id, action as 'overwrite' | 'skip' | 'keep_both', Boolean(input?.checked));
              }}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export function ContextMenu({
  x,
  y,
  visible,
  canPaste,
  onCommand,
}: {
  x: number;
  y: number;
  visible: boolean;
  canPaste: boolean;
  onCommand: (command: string) => void;
}) {
  if (!visible) return null;
  const items = ['open', 'open_new_tab', 'copy', 'cut', 'paste', 'rename', 'delete', 'delete_permanent', 'properties', 'copy_path'];
  return (
    <div className="sf-context-menu" style={{ left: x, top: y }}>
      {items.map((item) => (
        <button key={item} type="button" disabled={item === 'paste' && !canPaste} onClick={() => onCommand(item)}>
          {item.replaceAll('_', ' ')}
        </button>
      ))}
    </div>
  );
}

export function requestForTab(path: string, viewMode: ViewMode, knownHash?: string): ListChildrenRequest {
  return {
    path,
    knownHash,
    viewMode,
    sortKey: 'name',
    sortDirection: 'asc',
    filterText: '',
  };
}
