import type {
  Album,
  APIKey,
  AppConfig,
  Artist,
  AuthResponse,
  Author,
  Book,
  CardigannDefinition,
  DiscoveredPlugin,
  DomainClearance,
  PluginCatalogResponse,
  DownloadClient,
  Edition,
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
  SubtitleCandidate,
  SubtitlesStatus,
  SubtitleTrack,
  Tag,
  TagDetails,
  Track,
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

  // Music
  async getArtists(): Promise<Artist[]> {
    const res = await request<unknown>('/api/v1/artists');
    return unwrapArray<Artist>(res);
  },

  async lookupArtists(term: string): Promise<Artist[]> {
    const res = await request<unknown>(`/api/v1/artists/lookup?term=${encodeURIComponent(term)}`);
    return unwrapArray<Artist>(res);
  },

  async addArtist(payload: Partial<Artist>): Promise<Artist> {
    return request<Artist>('/api/v1/artists', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteArtist(id: number): Promise<void> {
    await request(`/api/v1/artists/${id}`, { method: 'DELETE' });
  },

  async getAlbums(artistId?: number): Promise<Album[]> {
    const q = artistId ? `?artistId=${artistId}` : '';
    const res = await request<unknown>(`/api/v1/albums${q}`);
    return unwrapArray<Album>(res);
  },

  async getTracks(opts?: { albumId?: number; artistId?: number }): Promise<Track[]> {
    const params = new URLSearchParams();
    if (opts?.albumId) params.set('albumId', String(opts.albumId));
    if (opts?.artistId) params.set('artistId', String(opts.artistId));
    const qs = params.toString();
    const res = await request<unknown>(`/api/v1/tracks${qs ? `?${qs}` : ''}`);
    return unwrapArray<Track>(res);
  },

  // Books
  async getAuthors(): Promise<Author[]> {
    const res = await request<unknown>('/api/v1/authors');
    return unwrapArray<Author>(res);
  },

  async lookupAuthors(term: string): Promise<Author[]> {
    const res = await request<unknown>(`/api/v1/authors/lookup?term=${encodeURIComponent(term)}`);
    return unwrapArray<Author>(res);
  },

  async addAuthor(payload: Partial<Author>): Promise<Author> {
    return request<Author>('/api/v1/authors', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteAuthor(id: number): Promise<void> {
    await request(`/api/v1/authors/${id}`, { method: 'DELETE' });
  },

  async getBooks(authorId?: number): Promise<Book[]> {
    const q = authorId ? `?authorId=${authorId}` : '';
    const res = await request<unknown>(`/api/v1/books${q}`);
    return unwrapArray<Book>(res);
  },

  async lookupBooks(term: string): Promise<Book[]> {
    const res = await request<unknown>(`/api/v1/books/lookup?term=${encodeURIComponent(term)}`);
    return unwrapArray<Book>(res);
  },

  async addBook(payload: Record<string, unknown>): Promise<{ book: Book; edition: Edition; authorId: number }> {
    return request('/api/v1/books', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async deleteBook(id: number): Promise<void> {
    await request(`/api/v1/books/${id}`, { method: 'DELETE' });
  },

  async getEditions(opts?: { bookId?: number; format?: string }): Promise<Edition[]> {
    const params = new URLSearchParams();
    if (opts?.bookId) params.set('bookId', String(opts.bookId));
    if (opts?.format) params.set('format', opts.format);
    const qs = params.toString();
    const res = await request<unknown>(`/api/v1/editions${qs ? `?${qs}` : ''}`);
    return unwrapArray<Edition>(res);
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

  async searchReleases(
    query: string,
    type = 'search',
    opts?: {
      season?: number;
      episode?: number;
      tvdbId?: number;
      tmdbId?: number;
      artistId?: number;
      albumId?: number;
      authorId?: number;
      bookId?: number;
      editionId?: number;
    }
  ): Promise<ReleaseResult[]> {
    const params = new URLSearchParams({ q: query, type });
    if (opts?.season != null && opts.season >= 0) {
      params.set('season', String(opts.season));
    }
    if (opts?.episode != null && opts.episode >= 0) {
      params.set('ep', String(opts.episode));
    }
    if (opts?.tvdbId != null && opts.tvdbId > 0) {
      params.set('tvdbId', String(opts.tvdbId));
    }
    if (opts?.tmdbId != null && opts.tmdbId > 0) {
      params.set('tmdbId', String(opts.tmdbId));
    }
    if (opts?.artistId) params.set('artistId', String(opts.artistId));
    if (opts?.albumId) params.set('albumId', String(opts.albumId));
    if (opts?.authorId) params.set('authorId', String(opts.authorId));
    if (opts?.bookId) params.set('bookId', String(opts.bookId));
    if (opts?.editionId) params.set('editionId', String(opts.editionId));
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
    episodeId?: number;
    seasonNumber?: number;
    artistId?: number;
    albumId?: number;
    authorId?: number;
    bookId?: number;
    editionId?: number;
  }): Promise<QueueItem> {
    return request<QueueItem>('/api/v1/releases/grab', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async manualRelease(payload: {
    title?: string;
    magnetUrl?: string;
    torrentBase64?: string;
    mediaType?: string;
    seriesId?: number;
    episodeId?: number;
    movieId?: number;
    seasonNumber?: number;
  }): Promise<QueueItem> {
    return request<QueueItem>('/api/v1/releases/manual', {
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

  async testNotificationPlugin(
    id: string,
    settings?: Record<string, unknown>
  ): Promise<{ ok: boolean; message: string; latencyMs: number }> {
    return request<{ ok: boolean; message: string; latencyMs: number }>(
      `/api/v1/plugins/${encodeURIComponent(id)}/test`,
      {
        method: 'POST',
        body: JSON.stringify({ settings: settings ?? {} }),
      }
    );
  },

  async getPluginCatalog(refresh = false): Promise<PluginCatalogResponse> {
    const q = refresh ? '?refresh=1' : '';
    return request<PluginCatalogResponse>(`/api/v1/plugins/catalog${q}`);
  },

  async refreshPluginCatalog(): Promise<PluginCatalogResponse> {
    return request<PluginCatalogResponse>('/api/v1/plugins/catalog/refresh', {
      method: 'POST',
    });
  },

  async installCatalogPlugin(id: string): Promise<{ ok: boolean; id: string }> {
    return request<{ ok: boolean; id: string }>('/api/v1/plugins/catalog/install', {
      method: 'POST',
      body: JSON.stringify({ id }),
    });
  },

  async uninstallCatalogPlugin(id: string): Promise<{ ok: boolean; id: string }> {
    return request<{ ok: boolean; id: string }>('/api/v1/plugins/catalog/uninstall', {
      method: 'POST',
      body: JSON.stringify({ id }),
    });
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

  async getMovieSubtitles(movieId: number): Promise<SubtitlesStatus> {
    return request<SubtitlesStatus>(`/api/v1/movies/${movieId}/subtitles`);
  },

  async searchMovieSubtitles(movieId: number, language?: string): Promise<SubtitleCandidate[]> {
    const q = language ? `?language=${encodeURIComponent(language)}` : '';
    const res = await request<unknown>(`/api/v1/movies/${movieId}/subtitles/search${q}`);
    return unwrapArray<SubtitleCandidate>(res);
  },

  async downloadMovieSubtitle(
    movieId: number,
    fileId: number,
    language: string
  ): Promise<SubtitleTrack> {
    return request<SubtitleTrack>(`/api/v1/movies/${movieId}/subtitles/download`, {
      method: 'POST',
      body: JSON.stringify({ fileId, language }),
    });
  },

  async ensureMovieSubtitles(movieId: number): Promise<SubtitleTrack[]> {
    const res = await request<unknown>(`/api/v1/movies/${movieId}/subtitles/ensure`, {
      method: 'POST',
      body: '{}',
    });
    return unwrapArray<SubtitleTrack>(res);
  },

  async getEpisodeSubtitles(episodeId: number): Promise<SubtitlesStatus> {
    return request<SubtitlesStatus>(`/api/v1/episodes/${episodeId}/subtitles`);
  },

  async searchEpisodeSubtitles(episodeId: number, language?: string): Promise<SubtitleCandidate[]> {
    const q = language ? `?language=${encodeURIComponent(language)}` : '';
    const res = await request<unknown>(`/api/v1/episodes/${episodeId}/subtitles/search${q}`);
    return unwrapArray<SubtitleCandidate>(res);
  },

  async downloadEpisodeSubtitle(
    episodeId: number,
    fileId: number,
    language: string
  ): Promise<SubtitleTrack> {
    return request<SubtitleTrack>(`/api/v1/episodes/${episodeId}/subtitles/download`, {
      method: 'POST',
      body: JSON.stringify({ fileId, language }),
    });
  },

  async ensureEpisodeSubtitles(episodeId: number): Promise<SubtitleTrack[]> {
    const res = await request<unknown>(`/api/v1/episodes/${episodeId}/subtitles/ensure`, {
      method: 'POST',
      body: '{}',
    });
    return unwrapArray<SubtitleTrack>(res);
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
    baseUrl: string;
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
