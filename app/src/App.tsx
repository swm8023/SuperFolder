import { useEffect, useMemo, useState } from 'react';
import { RpcClient, RpcClientSnapshot, RpcError } from './rpc/rpc';
import { SuperFolderApi } from './superfolder/api';
import {
  BrowserPane,
  ConflictDialog,
  ContextMenu,
  JobQueue,
  Sidebar,
  UtilityPanel,
  requestForTab,
} from './superfolder/components';
import { createInitialViewState, superFolderReducer, SuperFolderViewState } from './superfolder/state';
import { mapKeyboardShortcut, ShortcutCommand } from './superfolder/shortcuts';
import { BrowserTabState, DirectoryEntry, FavoriteItem, GitSummary, JobSnapshot, PaneState, PreviewResponse, SessionState, ViewMode } from './superfolder/types';

function asRpcError(error: unknown): RpcError {
  if (typeof error === 'object' && error !== null) {
    const value = error as Record<string, unknown>;
    if (typeof value.code === 'number' && typeof value.message === 'string') {
      return { code: value.code, message: value.message };
    }
  }
  return { code: 10000, message: error instanceof Error ? error.message : String(error) };
}

interface EditingState {
  path: string;
  name: string;
}

interface ContextMenuState {
  visible: boolean;
  x: number;
  y: number;
  paneId: string;
  path: string;
}

export default function App() {
  const client = useMemo(() => new RpcClient(), []);
  const api = useMemo(() => new SuperFolderApi(client), [client]);
  const [snapshot, setSnapshot] = useState<RpcClientSnapshot>(() => client.getSnapshot());
  const [view, setView] = useState<SuperFolderViewState | null>(null);
  const [favorites, setFavorites] = useState<FavoriteItem[]>([]);
  const [jobs, setJobs] = useState<JobSnapshot[]>([]);
  const [selectedByPane, setSelectedByPane] = useState<Record<string, string[]>>({});
  const [loadingPaths, setLoadingPaths] = useState<Record<string, boolean>>({});
  const [editing, setEditing] = useState<EditingState>({ path: '', name: '' });
  const [contextMenu, setContextMenu] = useState<ContextMenuState>({ visible: false, x: 0, y: 0, paneId: '', path: '' });
  const [gitSummary, setGitSummary] = useState<GitSummary | null>(null);
  const [preview, setPreview] = useState<PreviewResponse | null>(null);
  const [latestError, setLatestError] = useState<RpcError | null>(null);

  useEffect(() => {
    const stopState = client.onState((next) => {
      setSnapshot(next);
      if (next.latestError) setLatestError(next.latestError);
    });

    let alive = true;
    client
      .start()
      .then(async () => {
        if (!alive) return;
        const [{ session }, { favorites: loadedFavorites }, { jobs: loadedJobs }] = await Promise.all([
          api.getSession(),
          api.listFavorites(),
          api.listJobs(),
        ]);
        if (!alive) return;
        const initial = createInitialViewState(session);
        setView(initial);
        setFavorites(loadedFavorites);
        setJobs(loadedJobs);
        for (const pane of session.windows[0]?.panes ?? []) {
          const tab = activeTab(pane);
          if (tab) void loadChildrenForPath(tab.path, tab.viewMode, undefined);
        }
        const firstTab = activeTab(session.windows[0]?.panes[0]);
        if (firstTab) void refreshGit(firstTab.path);
      })
      .catch((error) => {
        if (alive) setLatestError(asRpcError(error));
      });

    const poll = window.setInterval(() => {
      if (client.status === 'connected') {
        void api.listJobs().then(({ jobs }) => setJobs(jobs)).catch((error) => setLatestError(asRpcError(error)));
      }
    }, 1000);

    return () => {
      alive = false;
      window.clearInterval(poll);
      stopState();
      client.stop();
    };
  }, [api, client]);

  async function persistSession(session: SessionState) {
    try {
      await api.updateSession(session);
    } catch (error) {
      setLatestError(asRpcError(error));
    }
  }

  async function loadChildrenForPath(path: string, viewMode: ViewMode, knownHash?: string) {
    setLoadingPaths((current) => ({ ...current, [path]: true }));
    try {
      const response = await api.listChildren(requestForTab(path, viewMode, knownHash));
      setView((current) => (current ? superFolderReducer(current, { type: 'childrenLoaded', response }) : current));
    } catch (error) {
      setLatestError(asRpcError(error));
    } finally {
      setLoadingPaths((current) => ({ ...current, [path]: false }));
    }
  }

  async function refreshGit(path: string) {
    try {
      await api.refreshGitStatus(path);
      window.setTimeout(async () => {
        try {
          const { summary } = await api.getGitSummary(path);
          setGitSummary(summary);
        } catch (error) {
          setLatestError(asRpcError(error));
        }
      }, 150);
    } catch (error) {
      setLatestError(asRpcError(error));
    }
  }

  function updateView(mutator: (current: SuperFolderViewState) => SuperFolderViewState) {
    setView((current) => {
      if (!current) return current;
      const next = mutator(current);
      void persistSession(next.session);
      return next;
    });
  }

  function navigatePane(paneId: string, tabId: string, path: string, title = leafName(path)) {
    updateView((current) => superFolderReducer(current, { type: 'tabNavigated', paneId, tabId, path, title }));
    setSelectedByPane((current) => ({ ...current, [paneId]: [] }));
    void loadChildrenForPath(path, activeTab(findPane(view?.session, paneId))?.viewMode ?? 'details');
    void refreshGit(path);
  }

  function changeViewMode(paneId: string, tabId: string, viewMode: ViewMode) {
    const currentView = view;
    if (!currentView) return;
    const tab = activeTab(findPane(view?.session, paneId));
    updateView((current) => superFolderReducer(current, { type: 'tabViewModeChanged', paneId, tabId, viewMode }));
    if (tab) void loadChildrenForPath(tab.path, viewMode, currentView.childrenByPath[tab.path]?.hash);
  }

  function updateUtility(update: (session: SessionState) => SessionState) {
    updateView((current) => ({ ...current, session: update(current.session) }));
  }

  async function handleEntryOpen(paneId: string, tabId: string, entry: DirectoryEntry) {
    if (entry.kind === 'directory') {
      navigatePane(paneId, tabId, entry.path, entry.name);
      return;
    }
    try {
      await api.openPath(entry.path);
    } catch (error) {
      setLatestError(asRpcError(error));
    }
  }

  function handleSelect(paneId: string, entry: DirectoryEntry, event: { ctrlKey?: boolean; shiftKey?: boolean }) {
    setSelectedByPane((current) => {
      const selected = current[paneId] ?? [];
      if (event.ctrlKey) {
        return { ...current, [paneId]: selected.includes(entry.path) ? selected.filter((path) => path !== entry.path) : [...selected, entry.path] };
      }
      return { ...current, [paneId]: [entry.path] };
    });
    if (entry.kind === 'file') {
      void api.getPreview(entry.path).then(({ preview }) => setPreview(preview)).catch(() => setPreview(null));
    } else {
      setPreview(null);
    }
  }

  async function runCommand(command: ShortcutCommand | string, paneId = activePaneId(), pathFromMenu = '') {
    const pane = findPane(view?.session, paneId);
    const tab = activeTab(pane);
    if (!pane || !tab) return;
    const selection = pathFromMenu ? [pathFromMenu] : selectedByPane[paneId] ?? [];
    const entries = view?.childrenByPath[tab.path]?.entries ?? [];
    const selectedEntry = entries.find((entry) => selection.includes(entry.path));
    setContextMenu((current) => ({ ...current, visible: false }));

    try {
      switch (command) {
        case 'open':
          if (selectedEntry) await handleEntryOpen(pane.id, tab.id, selectedEntry);
          return;
        case 'open_new_tab':
          if (selectedEntry?.kind === 'directory') addTab(pane.id, selectedEntry.path, selectedEntry.name);
          return;
        case 'rename':
          if (selectedEntry) setEditing({ path: selectedEntry.path, name: selectedEntry.name });
          return;
        case 'copy':
          if (selection.length) await api.setClipboard({ mode: 'copy', paths: selection, sourcePaneId: pane.id, sourceTabId: tab.id });
          return;
        case 'cut':
          if (selection.length) await api.setClipboard({ mode: 'cut', paths: selection, sourcePaneId: pane.id, sourceTabId: tab.id });
          return;
        case 'paste':
          await api.pasteClipboard(tab.path);
          await refreshJobs();
          return;
        case 'delete':
          if (selection.length) await api.executeMenu('delete', selection, tab.path);
          await refreshJobs();
          return;
        case 'deletePermanent':
        case 'delete_permanent':
          if (selection.length && window.confirm('Permanently delete selected items?')) await api.executeMenu('delete_permanent', selection, tab.path);
          await refreshJobs();
          return;
        case 'copy_path':
          if (selection.length) await navigator.clipboard?.writeText(selection.join('\n'));
          return;
        case 'focusPath':
          document.querySelector<HTMLInputElement>('.sf-pane.active input[name="path"], .sf-pathbar input[name="path"]')?.focus();
          return;
        case 'newTab':
          addTab(pane.id, tab.path, tab.title);
          return;
        case 'closeTab':
          closeTab(pane.id, tab.id);
          return;
        case 'up':
          navigatePane(pane.id, tab.id, parentPath(tab.path), leafName(parentPath(tab.path)));
          return;
        default:
          return;
      }
    } catch (error) {
      setLatestError(asRpcError(error));
    }
  }

  async function commitRename() {
    if (!editing.path || !editing.name.trim()) {
      setEditing({ path: '', name: '' });
      return;
    }
    const pane = findPane(view?.session, activePaneId());
    const tab = activeTab(pane);
    try {
      await api.executeMenu('rename', [editing.path], tab?.path ?? '', editing.name.trim());
      await refreshJobs();
      if (tab) window.setTimeout(() => void loadChildrenForPath(tab.path, tab.viewMode), 250);
    } catch (error) {
      setLatestError(asRpcError(error));
    } finally {
      setEditing({ path: '', name: '' });
    }
  }

  async function refreshJobs() {
    const { jobs } = await api.listJobs();
    setJobs(jobs);
  }

  function addTab(paneId: string, path: string, title: string) {
    updateView((current) => {
      const next = cloneView(current);
      const pane = findPane(next.session, paneId);
      if (!pane) return current;
      const id = `tab-${Date.now()}`;
      pane.tabs.push({ id, title, path, viewMode: 'details', sortKey: 'name', sortDirection: 'asc', filterText: '', expandedPaths: [] });
      pane.activeTabId = id;
      return next;
    });
    void loadChildrenForPath(path, 'details');
  }

  function closeTab(paneId: string, tabId: string) {
    updateView((current) => {
      const next = cloneView(current);
      const pane = findPane(next.session, paneId);
      if (!pane || pane.tabs.length <= 1) return current;
      pane.tabs = pane.tabs.filter((tab) => tab.id !== tabId);
      if (pane.activeTabId === tabId) pane.activeTabId = pane.tabs[0].id;
      return next;
    });
  }

  if (!snapshot.helloReady || !view) {
    return (
      <main className="loading-screen">
        <div className="loading-mark" aria-hidden="true" />
        <div className="loading-title">SuperFolder</div>
        <div className="loading-subtitle">Connecting</div>
      </main>
    );
  }

  const windowState = view.session.windows[0];
  const conflictJob = jobs.find((job) => job.status === 'waiting_conflict') ?? null;
  const activeSelectedPath = selectedByPane[activePaneId()]?.[0] ?? '';

  return (
    <main
      className="sf-app"
      tabIndex={0}
      onKeyDown={(event) => {
        const command = mapKeyboardShortcut(event);
        if (command) {
          event.preventDefault();
          void runCommand(command);
        }
      }}
      onClick={() => setContextMenu((current) => ({ ...current, visible: false }))}
    >
      <Sidebar favorites={favorites} onOpen={(path) => navigatePane(activePaneId(), activeTab(findPane(view.session, activePaneId()))?.id ?? '', path)} />
      <section className="sf-main">
        <header className="sf-topbar">
          <div>
            <h1>SuperFolder</h1>
            <span>{snapshot.status}</span>
          </div>
          {latestError ? <div className="sf-error">{latestError.message}</div> : null}
        </header>
        <section className="sf-panes">
          {windowState.panes.map((pane) => {
            const tab = activeTab(pane);
            const entries = tab ? view.childrenByPath[tab.path]?.entries ?? [] : [];
            return (
              <BrowserPane
                key={pane.id}
                pane={pane}
                entries={entries}
                loading={Boolean(tab && loadingPaths[tab.path])}
                selectedPaths={selectedByPane[pane.id] ?? []}
                editingPath={editing.path}
                editName={editing.name}
                onEditNameChange={(name) => setEditing((current) => ({ ...current, name }))}
                onCommitRename={commitRename}
                onCancelRename={() => setEditing({ path: '', name: '' })}
                onSelect={(entry, event) => handleSelect(pane.id, entry, event)}
                onOpen={(entry) => tab && void handleEntryOpen(pane.id, tab.id, entry)}
                onPathSubmit={(path) => tab && navigatePane(pane.id, tab.id, path)}
                onViewModeChange={(viewMode) => tab && changeViewMode(pane.id, tab.id, viewMode)}
                onRefresh={() => tab && void loadChildrenForPath(tab.path, tab.viewMode)}
                onNewTab={() => tab && addTab(pane.id, tab.path, tab.title)}
                onCloseTab={(tabId) => closeTab(pane.id, tabId)}
                onActivateTab={(tabId) => {
                  updateView((current) => {
                    const next = cloneView(current);
                    const targetPane = findPane(next.session, pane.id);
                    if (targetPane) targetPane.activeTabId = tabId;
                    return next;
                  });
                }}
                onContextMenu={(event, entry) => {
                  event.preventDefault();
                  if (entry) handleSelect(pane.id, entry, event);
                  setContextMenu({ visible: true, x: event.clientX, y: event.clientY, paneId: pane.id, path: entry?.path ?? '' });
                }}
              />
            );
          })}
        </section>
        <UtilityPanel
          session={view.session}
          gitSummary={gitSummary}
          preview={preview}
          selectedPath={activeSelectedPath}
          onTabChange={(tab) => updateUtility((session) => updateUtilityPanel(session, { activeTab: tab }))}
          onHeightChange={(height) => updateUtility((session) => updateUtilityPanel(session, { height }))}
          onToggleCollapsed={() => updateUtility((session) => updateUtilityPanel(session, { collapsed: !session.windows[0].utilityPanel.collapsed }))}
          onRefreshGit={() => {
            const pane = findPane(view.session, activePaneId());
            const tab = activeTab(pane);
            if (tab) void refreshGit(tab.path);
          }}
        />
      </section>
      <JobQueue jobs={jobs} onCancel={(jobId) => void api.cancelJob(jobId).then(refreshJobs)} />
      <ContextMenu visible={contextMenu.visible} x={contextMenu.x} y={contextMenu.y} canPaste={true} onCommand={(command) => void runCommand(command, contextMenu.paneId, contextMenu.path)} />
      <ConflictDialog
        job={conflictJob}
        onResolve={(jobId, action, applyToAll) => {
          void api.resolveConflict(jobId, action, applyToAll).then(refreshJobs);
        }}
      />
    </main>
  );

  function activePaneId() {
    return view?.session.windows[0]?.activePaneId || view?.session.windows[0]?.panes[0]?.id || '';
  }
}

function activeTab(pane?: PaneState | null): BrowserTabState | null {
  return pane?.tabs.find((tab) => tab.id === pane.activeTabId) ?? pane?.tabs[0] ?? null;
}

function findPane(session: SessionState | undefined, paneId: string) {
  return session?.windows[0]?.panes.find((pane) => pane.id === paneId) ?? null;
}

function updateUtilityPanel(session: SessionState, patch: Partial<SessionState['windows'][number]['utilityPanel']>) {
  const next = JSON.parse(JSON.stringify(session)) as SessionState;
  next.windows[0].utilityPanel = { ...next.windows[0].utilityPanel, ...patch };
  return next;
}

function cloneView(view: SuperFolderViewState): SuperFolderViewState {
  return JSON.parse(JSON.stringify(view)) as SuperFolderViewState;
}

function leafName(path: string) {
  const normalized = path.replaceAll('/', '\\');
  const parts = normalized.split('\\').filter(Boolean);
  return parts.at(-1) ?? path;
}

function parentPath(path: string) {
  const normalized = path.replaceAll('/', '\\');
  const match = normalized.match(/^([A-Za-z]:\\)$/);
  if (match) return normalized;
  const index = normalized.lastIndexOf('\\');
  if (index <= 2) return normalized.slice(0, 3);
  return normalized.slice(0, index);
}
