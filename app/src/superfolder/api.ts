import { rpc } from '../rpc/rpc';
import {
  ClipboardState,
  FavoriteItem,
  GitSummary,
  JobSnapshot,
  ListChildrenRequest,
  ListChildrenResponse,
  PreviewResponse,
  SessionState,
} from './types';

export interface RpcCaller {
  call(method: number, payload?: unknown): Promise<unknown>;
}

export class SuperFolderApi {
  constructor(private readonly rpcClient: RpcCaller) {}

  getSession(): Promise<{ session: SessionState }> {
    return this.rpcClient.call(rpc.folder.session.get, {}) as Promise<{ session: SessionState }>;
  }

  updateSession(session: SessionState): Promise<{ session: SessionState }> {
    return this.rpcClient.call(rpc.folder.session.update, { session }) as Promise<{ session: SessionState }>;
  }

  listFavorites(): Promise<{ favorites: FavoriteItem[] }> {
    return this.rpcClient.call(rpc.folder.favorites.list, {}) as Promise<{ favorites: FavoriteItem[] }>;
  }

  updateFavorites(favorites: FavoriteItem[]): Promise<{ favorites: FavoriteItem[] }> {
    return this.rpcClient.call(rpc.folder.favorites.update, { favorites }) as Promise<{ favorites: FavoriteItem[] }>;
  }

  listChildren(request: ListChildrenRequest): Promise<ListChildrenResponse> {
    return this.rpcClient.call(rpc.folder.children.list, request) as Promise<ListChildrenResponse>;
  }

  openPath(path: string): Promise<{ opened: string }> {
    return this.rpcClient.call(rpc.folder.open, { path }) as Promise<{ opened: string }>;
  }

  executeMenu(command: string, selection: string[], targetDir = '', newName = ''): Promise<unknown> {
    return this.rpcClient.call(rpc.folder.menu.execute, { command, selection, targetDir, newName });
  }

  setClipboard(clipboard: ClipboardState): Promise<{ clipboard: ClipboardState }> {
    return this.rpcClient.call(rpc.folder.clipboard.set, clipboard) as Promise<{ clipboard: ClipboardState }>;
  }

  pasteClipboard(targetDir: string): Promise<{ job: JobSnapshot }> {
    return this.rpcClient.call(rpc.folder.clipboard.paste, { targetDir }) as Promise<{ job: JobSnapshot }>;
  }

  listJobs(): Promise<{ jobs: JobSnapshot[] }> {
    return this.rpcClient.call(rpc.job.list, {}) as Promise<{ jobs: JobSnapshot[] }>;
  }

  cancelJob(jobId: string): Promise<{ jobId: string }> {
    return this.rpcClient.call(rpc.job.cancel, { jobId }) as Promise<{ jobId: string }>;
  }

  resolveConflict(jobId: string, action: 'overwrite' | 'skip' | 'keep_both', applyToAll: boolean): Promise<{ jobId: string }> {
    return this.rpcClient.call(rpc.job.conflict.resolve, { jobId, action, applyToAll }) as Promise<{ jobId: string }>;
  }

  refreshGitStatus(path: string): Promise<{ path: string }> {
    return this.rpcClient.call(rpc.git.status.refresh, { path }) as Promise<{ path: string }>;
  }

  getGitSummary(path: string): Promise<{ summary: GitSummary }> {
    return this.rpcClient.call(rpc.git.summary.get, { path }) as Promise<{ summary: GitSummary }>;
  }

  getPreview(path: string): Promise<{ preview: PreviewResponse }> {
    return this.rpcClient.call(rpc.preview.get, { path }) as Promise<{ preview: PreviewResponse }>;
  }
}
