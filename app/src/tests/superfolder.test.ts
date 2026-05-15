import { describe, expect, test } from 'vitest';
import { rpc } from '../rpc/rpc';
import { SuperFolderApi } from '../superfolder/api';
import { createInitialViewState, superFolderReducer } from '../superfolder/state';
import { mapKeyboardShortcut } from '../superfolder/shortcuts';
import { SessionState, ViewMode } from '../superfolder/types';

const session: SessionState = {
  version: 1,
  windows: [
    {
      id: 'window-1',
      activePaneId: 'pane-left',
      utilityPanel: { collapsed: false, height: 240, activeTab: 'preview' },
      panes: [
        {
          id: 'pane-left',
          activeTabId: 'tab-left',
          tabs: [
            {
              id: 'tab-left',
              title: 'Home',
              path: 'C:\\Users\\me',
              viewMode: 'details',
              sortKey: 'name',
              sortDirection: 'asc',
              filterText: '',
              expandedPaths: [],
            },
          ],
        },
      ],
    },
  ],
};

describe('superFolderReducer', () => {
  test('stores changed children by path and keeps existing children on unchanged response', () => {
    const state = createInitialViewState(session);
    const loaded = superFolderReducer(state, {
      type: 'childrenLoaded',
      response: {
        path: 'C:\\Users\\me',
        unchanged: false,
        childrenHash: 'hash-1',
        entries: [{ name: 'file.txt', path: 'C:\\Users\\me\\file.txt', kind: 'file', size: 1, mtime: 1, readonly: false, hidden: false, system: false, hasChildren: false }],
      },
    });

    expect(loaded.childrenByPath['C:\\Users\\me'].entries).toHaveLength(1);

    const unchanged = superFolderReducer(loaded, {
      type: 'childrenLoaded',
      response: { path: 'C:\\Users\\me', unchanged: true, childrenHash: 'hash-1' },
    });

    expect(unchanged.childrenByPath['C:\\Users\\me'].entries).toEqual(loaded.childrenByPath['C:\\Users\\me'].entries);
  });

  test('updates active tab path and view mode through reducer actions', () => {
    const state = createInitialViewState(session);
    const navigated = superFolderReducer(state, {
      type: 'tabNavigated',
      paneId: 'pane-left',
      tabId: 'tab-left',
      path: 'D:\\Work',
      title: 'Work',
    });
    const changedView = superFolderReducer(navigated, {
      type: 'tabViewModeChanged',
      paneId: 'pane-left',
      tabId: 'tab-left',
      viewMode: 'tree',
    });

    const tab = changedView.session.windows[0].panes[0].tabs[0];
    expect(tab.path).toBe('D:\\Work');
    expect(tab.title).toBe('Work');
    expect(tab.viewMode satisfies ViewMode).toBe('tree');
  });
});

describe('mapKeyboardShortcut', () => {
  test('maps explorer keyboard shortcuts to commands', () => {
    expect(mapKeyboardShortcut({ key: 'Enter' })).toBe('open');
    expect(mapKeyboardShortcut({ key: 'F2' })).toBe('rename');
    expect(mapKeyboardShortcut({ key: 'Delete' })).toBe('delete');
    expect(mapKeyboardShortcut({ key: 'Delete', shiftKey: true })).toBe('deletePermanent');
    expect(mapKeyboardShortcut({ key: 'c', ctrlKey: true })).toBe('copy');
    expect(mapKeyboardShortcut({ key: 'x', ctrlKey: true })).toBe('cut');
    expect(mapKeyboardShortcut({ key: 'v', ctrlKey: true })).toBe('paste');
    expect(mapKeyboardShortcut({ key: 'l', ctrlKey: true })).toBe('focusPath');
    expect(mapKeyboardShortcut({ key: 't', ctrlKey: true })).toBe('newTab');
    expect(mapKeyboardShortcut({ key: 'w', ctrlKey: true })).toBe('closeTab');
    expect(mapKeyboardShortcut({ key: 'ArrowLeft', altKey: true })).toBe('historyBack');
    expect(mapKeyboardShortcut({ key: 'ArrowRight', altKey: true })).toBe('historyForward');
    expect(mapKeyboardShortcut({ key: 'Backspace' })).toBe('up');
  });
});

describe('SuperFolderApi', () => {
  test('api wrapper calls generated folder methods', async () => {
    const calls: Array<{ method: number; payload: unknown }> = [];
    const api = new SuperFolderApi({
      call: async (method: number, payload: unknown) => {
        calls.push({ method, payload });
        return { ok: true };
      },
    });

    await api.getSession();
    await api.listChildren({ path: 'C:\\Users\\me', knownHash: 'h', viewMode: 'details', sortKey: 'name', sortDirection: 'asc', filterText: '' });
    await api.setClipboard({ mode: 'copy', paths: ['C:\\a.txt'], sourcePaneId: 'pane-left', sourceTabId: 'tab-left' });

    expect(calls).toEqual([
      { method: rpc.folder.session.get, payload: {} },
      { method: rpc.folder.children.list, payload: { path: 'C:\\Users\\me', knownHash: 'h', viewMode: 'details', sortKey: 'name', sortDirection: 'asc', filterText: '' } },
      { method: rpc.folder.clipboard.set, payload: { mode: 'copy', paths: ['C:\\a.txt'], sourcePaneId: 'pane-left', sourceTabId: 'tab-left' } },
    ]);
  });
});
