import type {
  APIKey,
  AppConfig,
  AuthResponse,
  CardigannDefinition,
  DiscoveredPlugin,
  DomainClearance,
  DownloadClient,
  Episode,
  FlareSolverrHealth,
  Indexer,
  IndexerProxy,
  Movie,
  OverviewResponse,
  QualityDefinition,
  QualityProfile,
  QueueItem,
  ReleaseResult,
  Role,
  RootFolder,
  Series,
  Tag,
  TagDetails,
  User,
} from './types';

const TOKEN_STORAGE_KEY = 'replacearr_token';

export function getStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function setStoredToken(token: string | null): void {
  try {
    if (token) {
      localStorage.setItem(TOKEN_STORAGE_KEY, token);
    } else {
      localStorage.removeItem(TOKEN_STORAGE_KEY);
    }
  } catch {
    // Ignore storage errors
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  };

  if (options.body && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(path, {
    ...options,
    headers,
    credentials: 'same-origin',
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  if (!res.ok) {
    const errObj = data as { error?: string; message?: string } | null;
    const msg =
      errObj?.error ||
      errObj?.message ||
      (typeof data === 'string' && data.length < 200 ? data : `Request failed (${res.status})`);
    throw new Error(msg);
  }

  return data as T;
}

function unwrapArray<T>(val: unknown, fallbackKey?: string): T[] {
  if (Array.isArray(val)) return val as T[];
  if (val && typeof val === 'object') {
    const obj = val as Record<string, unknown>;
    if (fallbackKey && Array.isArray(obj[fallbackKey])) {
      return obj[fallbackKey] as T[];
    }
    for (const k of ['items', 'records', 'results', 'definitions', 'permissions', 'data']) {
      if (Array.isArray(obj[k])) {
        return obj[k] as T[];
      }
    }
  }
  return [];
}

export const api = {
  // Auth
  async login(username: string, password: string): Promise<AuthResponse> {
    const res = await request<AuthResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    if (res.token) {
      setStoredToken(res.token);
    }
    return res;
  },

  async logout(): Promise<void> {
    try {
      await request('/api/v1/auth/logout', { method: 'POST' });
    } finally {
      setStoredToken(null);
    }
  },

  async me(): Promise<AuthResponse> {
    return request<AuthResponse>('/api/v1/auth/me');
  },

  // Overview
  async getOverview(): Promise<OverviewResponse> {
    return request<OverviewResponse>('/api/v1/overview');
  },

  // TV Series (Sonarr)
  async getSeries(): Promise<Series[]> {
    const res = await request<unknown>('/api/v1/series');
    return unwrapArray<Series>(res);
  },

  async lookupSeries(term: string): Promise<Series[]> {
    const res = await request<unknown>(`/api/v1/series/lookup?term=${encodeURIComponent(term)}`);
    return unwrapArray<Series>(res);
  },

  async addSeries(payload: Partial<Series>): Promise<Series> {
    return request<Series>('/api/v1/series', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteSeries(id: number): Promise<void> {
    await request(`/api/v1/series/${id}`, { method: 'DELETE' });
  },

  async getEpisodes(seriesId: number): Promise<Episode[]> {
    const res = await request<unknown>(`/api/v1/episodes?seriesId=${seriesId}`);
    return unwrapArray<Episode>(res);
  },

  // Movies (Radarr)
  async getMovies(): Promise<Movie[]> {
    const res = await request<unknown>('/api/v1/movies');
    return unwrapArray<Movie>(res);
  },

  async lookupMovies(term: string): Promise<Movie[]> {
    const res = await request<unknown>(`/api/v1/movies/lookup?term=${encodeURIComponent(term)}`);
    return unwrapArray<Movie>(res);
  },

  async addMovie(payload: Partial<Movie>): Promise<Movie> {
    return request<Movie>('/api/v1/movies', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteMovie(id: number): Promise<void> {
    await request(`/api/v1/movies/${id}`, { method: 'DELETE' });
  },

  // Indexers & Unified Search (Prowlarr)
  async getIndexers(): Promise<Indexer[]> {
    const res = await request<unknown>('/api/v1/indexers');
    return unwrapArray<Indexer>(res);
  },

  async addIndexer(payload: Partial<Indexer>): Promise<Indexer> {
    return request<Indexer>('/api/v1/indexers', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteIndexer(id: number): Promise<void> {
    await request(`/api/v1/indexers/${id}`, { method: 'DELETE' });
  },

  async getCardigannDefinitions(): Promise<CardigannDefinition[]> {
    const res = await request<unknown>('/api/v1/indexers/definitions');
    return unwrapArray<CardigannDefinition>(res, 'definitions');
  },

  // FlareSolverr & Indexer Proxies
  async getIndexerProxies(): Promise<IndexerProxy[]> {
    const res = await request<unknown>('/api/v1/indexerproxies');
    return unwrapArray<IndexerProxy>(res);
  },

  async addIndexerProxy(payload: Partial<IndexerProxy>): Promise<IndexerProxy> {
    return request<IndexerProxy>('/api/v1/indexerproxies', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async updateIndexerProxy(id: number, payload: Partial<IndexerProxy>): Promise<IndexerProxy> {
    return request<IndexerProxy>(`/api/v1/indexerproxies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async deleteIndexerProxy(id: number): Promise<void> {
    await request(`/api/v1/indexerproxies/${id}`, { method: 'DELETE' });
  },

  async testIndexerProxy(host: string): Promise<FlareSolverrHealth> {
    return request<FlareSolverrHealth>('/api/v1/indexerproxies/test', {
      method: 'POST',
      body: JSON.stringify({ host }),
    });
  },

  async solveWithFlareSolverr(payload: {
    host?: string;
    url: string;
    requestTimeout?: number;
    sessionTtlMinutes?: number;
    disableMedia?: boolean;
  }): Promise<Record<string, unknown>> {
    return request<Record<string, unknown>>('/api/v1/flaresolverr/solve', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async getClearances(): Promise<DomainClearance[]> {
    const res = await request<unknown>('/api/v1/flaresolverr/clearances');
    return unwrapArray<DomainClearance>(res);
  },

  async clearClearances(): Promise<void> {
    await request('/api/v1/flaresolverr/clearances', { method: 'DELETE' });
  },

  async searchReleases(query: string, type = 'search'): Promise<ReleaseResult[]> {
    const params = new URLSearchParams({ q: query, type });
    const res = await request<unknown>(`/api/v1/search?${params.toString()}`);
    return unwrapArray<ReleaseResult>(res, 'results');
  },

  async grabRelease(payload: {
    title: string;
    downloadUrl: string;
    protocol: string;
    indexer: string;
    size: number;
    mediaType?: string;
    seriesId?: number;
    movieId?: number;
  }): Promise<QueueItem> {
    return request<QueueItem>('/api/v1/releases/grab', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  // Queue, Download Clients, Profiles, Root Folders
  async getQueue(): Promise<QueueItem[]> {
    const res = await request<unknown>('/api/v1/queue');
    return unwrapArray<QueueItem>(res, 'records');
  },

  async removeQueueItem(id: number): Promise<void> {
    await request(`/api/v1/queue/${id}`, { method: 'DELETE' });
  },

  async getDownloadClients(): Promise<DownloadClient[]> {
    const res = await request<unknown>('/api/v1/downloadclients');
    return unwrapArray<DownloadClient>(res);
  },

  async addDownloadClient(payload: Partial<DownloadClient>): Promise<DownloadClient> {
    return request<DownloadClient>('/api/v1/downloadclients', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteDownloadClient(id: number): Promise<void> {
    await request(`/api/v1/downloadclients/${id}`, { method: 'DELETE' });
  },

  async getProfiles(): Promise<QualityProfile[]> {
    const res = await request<unknown>('/api/v1/profiles');
    return unwrapArray<QualityProfile>(res);
  },

  async getRootFolders(): Promise<RootFolder[]> {
    const res = await request<unknown>('/api/v1/rootfolders');
    return unwrapArray<RootFolder>(res);
  },

  async addRootFolder(payload: { path: string; mediaType: string }): Promise<RootFolder> {
    return request<RootFolder>('/api/v1/rootfolders', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  // Go Plugin System
  async getPlugins(): Promise<DiscoveredPlugin[]> {
    const res = await request<unknown>('/api/v1/plugins');
    return unwrapArray<DiscoveredPlugin>(res, 'plugins');
  },

  async updatePlugin(
    id: string,
    payload: { enabled: boolean; settings: Record<string, unknown> }
  ): Promise<DiscoveredPlugin> {
    return request<DiscoveredPlugin>(`/api/v1/plugins/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async reloadPlugins(): Promise<DiscoveredPlugin[]> {
    const res = await request<unknown>('/api/v1/plugins/reload', {
      method: 'POST',
    });
    return unwrapArray<DiscoveredPlugin>(res, 'plugins');
  },

  // Users, Roles, API Keys, Permissions
  async getUsers(): Promise<User[]> {
    const res = await request<unknown>('/api/v1/users');
    return unwrapArray<User>(res);
  },

  async createUser(payload: {
    username: string;
    email: string;
    password: string;
    roleId: number;
  }): Promise<User> {
    return request<User>('/api/v1/users', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async updateUser(
    id: number,
    payload: { email: string; roleId: number; isActive: boolean; password?: string }
  ): Promise<User> {
    return request<User>(`/api/v1/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async deleteUser(id: number): Promise<void> {
    await request(`/api/v1/users/${id}`, { method: 'DELETE' });
  },

  async getRoles(): Promise<Role[]> {
    const res = await request<unknown>('/api/v1/roles');
    return unwrapArray<Role>(res);
  },

  async createRole(payload: {
    name: string;
    description: string;
    permissions: string[];
  }): Promise<Role> {
    return request<Role>('/api/v1/roles', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async updateRole(
    id: number,
    payload: { description: string; permissions: string[] }
  ): Promise<Role> {
    return request<Role>(`/api/v1/roles/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async getApiKeys(): Promise<APIKey[]> {
    const res = await request<unknown>('/api/v1/apikeys');
    return unwrapArray<APIKey>(res);
  },

  async createApiKey(payload: {
    name: string;
    scope: string;
    permissions?: string[];
  }): Promise<APIKey> {
    return request<APIKey>('/api/v1/apikeys', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteApiKey(id: number): Promise<void> {
    await request(`/api/v1/apikeys/${id}`, { method: 'DELETE' });
  },

  async getPermissions(): Promise<string[]> {
    const res = await request<unknown>('/api/v1/permissions');
    return unwrapArray<string>(res, 'permissions');
  },

  async getConfig(): Promise<AppConfig> {
    return request<AppConfig>('/api/v1/config');
  },

  async saveConfig(payload: AppConfig): Promise<AppConfig> {
    return request<AppConfig>('/api/v1/config', {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async regenerateApiKey(): Promise<{ apiKey: string }> {
    return request<{ apiKey: string }>('/api/v1/config/apikey', { method: 'POST' });
  },

  async getQualities(): Promise<QualityDefinition[]> {
    const res = await request<unknown>('/api/v1/qualities');
    return unwrapArray<QualityDefinition>(res);
  },

  async createProfile(payload: Omit<QualityProfile, 'id'>): Promise<QualityProfile> {
    return request<QualityProfile>('/api/v1/profiles', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async updateProfile(id: number, payload: Partial<QualityProfile>): Promise<QualityProfile> {
    return request<QualityProfile>(`/api/v1/profiles/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async deleteProfile(id: number): Promise<void> {
    await request(`/api/v1/profiles/${id}`, { method: 'DELETE' });
  },

  async getTags(): Promise<Tag[]> {
    const res = await request<unknown>('/api/v1/tags');
    return unwrapArray<Tag>(res);
  },

  async createTag(label: string): Promise<Tag> {
    return request<Tag>('/api/v1/tags', {
      method: 'POST',
      body: JSON.stringify({ label }),
    });
  },

  async deleteTag(id: number): Promise<void> {
    await request(`/api/v1/tags/${id}`, { method: 'DELETE' });
  },

  async getTagDetails(id: number): Promise<TagDetails> {
    return request<TagDetails>(`/api/v1/tags/${id}`);
  },

  async deleteRootFolder(id: number): Promise<void> {
    await request(`/api/v1/rootfolders/${id}`, { method: 'DELETE' });
  },

  async updateIndexer(id: number, payload: Partial<Indexer>): Promise<Indexer> {
    return request<Indexer>(`/api/v1/indexers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async updateDownloadClient(id: number, payload: Partial<DownloadClient>): Promise<DownloadClient> {
    return request<DownloadClient>(`/api/v1/downloadclients/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  },

  async testDownloadClient(payload: {
    host: string;
    port: number;
    useSsl: boolean;
  }): Promise<{ ok: boolean; message: string; latencyMs: number }> {
    return request<{ ok: boolean; message: string; latencyMs: number }>('/api/v1/downloadclients/test', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },
};

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let val = bytes;
  let idx = 0;
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024;
    idx++;
  }
  return `${val.toFixed(idx > 1 ? 2 : 0)} ${units[idx]}`;
}
