import { BrowserTabState, DirectoryEntry, ListChildrenResponse, SessionState, ViewMode } from './types';

export interface ChildrenState {
  hash: string;
  entries: DirectoryEntry[];
}

export interface SuperFolderViewState {
  session: SessionState;
  childrenByPath: Record<string, ChildrenState>;
}

export type SuperFolderAction =
  | { type: 'childrenLoaded'; response: ListChildrenResponse }
  | { type: 'tabNavigated'; paneId: string; tabId: string; path: string; title: string }
  | { type: 'tabViewModeChanged'; paneId: string; tabId: string; viewMode: ViewMode };

export function createInitialViewState(session: SessionState): SuperFolderViewState {
  return {
    session: cloneSession(session),
    childrenByPath: {},
  };
}

export function superFolderReducer(state: SuperFolderViewState, action: SuperFolderAction): SuperFolderViewState {
  switch (action.type) {
    case 'childrenLoaded':
      return reduceChildrenLoaded(state, action.response);
    case 'tabNavigated':
      return updateTab(state, action.paneId, action.tabId, (tab) => ({ ...tab, path: action.path, title: action.title }));
    case 'tabViewModeChanged':
      return updateTab(state, action.paneId, action.tabId, (tab) => ({ ...tab, viewMode: action.viewMode }));
    default:
      return state;
  }
}

function reduceChildrenLoaded(state: SuperFolderViewState, response: ListChildrenResponse): SuperFolderViewState {
  const current = state.childrenByPath[response.path];
  if (response.unchanged && current) {
    return {
      ...state,
      childrenByPath: {
        ...state.childrenByPath,
        [response.path]: { ...current, hash: response.childrenHash },
      },
    };
  }
  return {
    ...state,
    childrenByPath: {
      ...state.childrenByPath,
      [response.path]: { hash: response.childrenHash, entries: response.entries ?? [] },
    },
  };
}

function updateTab(
  state: SuperFolderViewState,
  paneId: string,
  tabId: string,
  update: (tab: BrowserTabState) => BrowserTabState,
): SuperFolderViewState {
  return {
    ...state,
    session: {
      ...state.session,
      windows: state.session.windows.map((window) => ({
        ...window,
        panes: window.panes.map((pane) => {
          if (pane.id !== paneId) {
            return pane;
          }
          return {
            ...pane,
            tabs: pane.tabs.map((tab) => (tab.id === tabId ? update(tab) : tab)),
          };
        }),
      })),
    },
  };
}

function cloneSession(session: SessionState): SessionState {
  return JSON.parse(JSON.stringify(session)) as SessionState;
}
