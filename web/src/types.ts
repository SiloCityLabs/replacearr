/** @deprecated Prefer NavView from nav.ts — kept for Dashboard shortcuts */
export type NavTab =
  | 'dashboard'
  | 'calendar'
  | 'series'
  | 'movies'
  | 'music'
  | 'books'
  | 'indexers'
  | 'queue'
  | 'plugins'
  | 'users'
  | 'settings'
  | 'system';

export interface BlocklistItem {
  id: number;
  mediaType: string;
  sourceTitle: string;
  torrentInfoHash?: string;
  indexer?: string;
  message?: string;
  seriesId?: number;
  movieId?: number;
  artistId?: number;
  authorId?: number;
  size?: number;
  protocol?: string;
  createdAt: string;
}

export interface NamedCount {
  name: string;
  count: number;
}

export interface MediaStatistics {
  mediaType: string;
  itemCount: number;
  monitoredCount: number;
  withFileCount: number;
  missingCount: number;
  unreleasedCount: number;
  sizeOnDisk: number;
  averageFileSize: number;
  statusBreakdown?: NamedCount[];
  seriesTypeBreakdown?: NamedCount[];
  episodeStatusBreakdown?: NamedCount[];
  qualityProfiles?: NamedCount[];
  qualityBreakdown?: Array<{ name: string; count: number; size: number }>;
  genres?: NamedCount[];
  fileTypes?: NamedCount[];
  missingItems?: Array<{ title: string; detail?: string; id?: number }>;
}

export interface CalendarEvent {
  id: string;
  title: string;
  date: string;
  mediaType: string;
  status: 'downloaded' | 'missing' | 'queued' | 'unreleased' | string;
  seriesId?: number;
  movieId?: number;
  artistId?: number;
  albumId?: number;
  bookId?: number;
  episodeId?: number;
}

export interface HealthCheck {
  source: string;
  type: 'ok' | 'warning' | 'error' | string;
  message: string;
  time?: string;
}

export interface SystemTask {
  name: string;
  label?: string;
  interval?: string;
  lastRunAt?: string;
  nextRunAt?: string;
  lastDurationMs?: number;
  status?: string;
}

export interface BackupFile {
  id: string;
  name: string;
  size: number;
  createdAt: string;
}

export interface UpdateStatus {
  currentVersion: string;
  latestVersion?: string;
  updateAvailable: boolean;
  mechanism: string;
  checkedAt?: string;
}

export interface HistoryRecord {
  id: number;
  mediaType: string;
  seriesId?: number;
  episodeId?: number;
  movieId?: number;
  artistId?: number;
  albumId?: number;
  authorId?: number;
  bookId?: number;
  editionId?: number;
  sourceTitle: string;
  quality?: Record<string, unknown>;
  date: string;
  eventType: string;
  data?: Record<string, unknown>;
}

export interface HistoryPage {
  page: number;
  pageSize: number;
  totalRecords: number;
  records: HistoryRecord[];
}

export interface LogEntry {
  time: string;
  level: string;
  logger?: string;
  message: string;
}

export interface ChangelogSection {
  version: string;
  date?: string;
  notes: string[];
}

export interface LibraryImportMatch {
  libraryId?: number;
  tmdbId?: number;
  tvdbId?: number;
  imdbId?: string;
  foreignId?: string;
  authorName?: string;
  format?: string;
  title: string;
  year?: number;
  overview?: string;
  disambiguation?: string;
  inLibrary?: boolean;
}

export interface LibraryImportScanItem {
  path: string;
  relativePath: string;
  size: number;
  guessedTitle: string;
  guessedYear?: number;
  seasonNumber?: number;
  episodeNumber?: number;
  matches?: LibraryImportMatch[];
  match?: LibraryImportMatch | null;
}

export interface LibraryImportScanResult {
  scanId?: string;
  total: number;
  offset: number;
  limit: number;
  pageSize: number;
  items: LibraryImportScanItem[];
}

export interface LibraryImportItemResult {
  path: string;
  ok: boolean;
  error?: string;
}

export interface LibraryImportResult {
  imported: number;
  results?: LibraryImportItemResult[];
}

/** Interactive Search payload — must target a library entity. */
export interface ReleaseSearchRequest {
  query: string;
  type?: string;
  seriesId?: number;
  movieId?: number;
  episodeId?: number;
  artistId?: number;
  albumId?: number;
  authorId?: number;
  bookId?: number;
  editionId?: number;
  tvdbId?: number;
  tmdbId?: number;
  season?: number;
  episode?: number;
  contextLabel?: string;
}

export interface BrowserNotificationEvent {
  eventType: string;
  payload: Record<string, unknown>;
  sentAt: string;
}

export interface Role {
  id: number;
  name: string;
  description: string;
  permissions: string[];
  builtIn: boolean;
  createdAt: string;
}

export interface User {
  id: number;
  username: string;
  email: string;
  roleId: number;
  roleName: string;
  permissions: string[];
  isActive: boolean;
  lastLoginAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface APIKey {
  id: number;
  name: string;
  keyPrefix: string;
  rawKey?: string;
  userId: number;
  scope: 'all' | 'sonarr' | 'radarr' | 'prowlarr' | string;
  permissions: string[];
  lastUsedAt?: string;
  createdAt: string;
}

export interface AuthResponse {
  token?: string;
  user: User;
  masterApiKey: string;
}

export interface AuthSession {
  id: string;
  userId: number;
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
  revokedAt?: string;
  rememberMe: boolean;
  userAgent: string;
  ipAddress: string;
  current?: boolean;
}

export interface MediaImage {
  coverType: string;
  url: string;
  remoteUrl: string;
}

export interface Season {
  seasonNumber: number;
  monitored: boolean;
  statistics?: Record<string, unknown>;
}

export interface Series {
  id: number;
  title: string;
  sortTitle: string;
  status: string;
  ended: boolean;
  overview: string;
  network: string;
  airTime: string;
  images: MediaImage[];
  seasons: Season[];
  year: number;
  path: string;
  qualityProfileId: number;
  languageProfileId: number;
  seasonFolder: boolean;
  monitored: boolean;
  useSceneNumbering: boolean;
  runtime: number;
  tvdbId: number;
  tvRageId?: number;
  tvMazeId?: number;
  imdbId: string;
  titleSlug: string;
  rootFolderPath: string;
  seriesType: string;
  cleanTitle: string;
  genres: string[];
  tags: number[];
  added: string;
  statistics?: Record<string, unknown>;
}

export interface Episode {
  id: number;
  seriesId: number;
  tvdbId: number;
  episodeFileId: number;
  seasonNumber: number;
  episodeNumber: number;
  title: string;
  airDate: string;
  airDateUtc: string;
  overview: string;
  hasFile: boolean;
  monitored: boolean;
  absoluteEpisodeNumber?: number;
}

export interface MediaFileInfo {
  id: number;
  path: string;
  relativePath: string;
  size: number;
  sceneName: string;
  releaseGroup: string;
  quality?: string;
  resolution?: number;
  videoCodec?: string;
  audioCodec?: string;
  languages?: string;
  indexer?: string;
  dateAdded: string;
  seriesId?: number;
  episodeId?: number;
  movieId?: number;
}

export interface MediaFileUpdate {
  quality?: string;
  languages?: string;
  indexer?: string;
  releaseGroup?: string;
  sceneName?: string;
  videoCodec?: string;
  audioCodec?: string;
  resolution?: number;
}

export interface FileDetailProperty {
  label: string;
  value: string;
}

export interface FileDetails {
  path: string;
  kind: string;
  source?: string;
  properties: FileDetailProperty[];
}

export interface SubtitleTrack {
  id: number;
  mediaType: string;
  movieId?: number;
  episodeId?: number;
  episodeFileId?: number;
  movieFileId?: number;
  language: string;
  path: string;
  provider: string;
  releaseName: string;
  score: number;
  forced: boolean;
  hearingImpaired: boolean;
  downloadedAt: string;
}

export interface SubtitleCandidate {
  provider: string;
  fileId: number;
  language: string;
  releaseName: string;
  hearingImpaired: boolean;
  forced: boolean;
  hashMatch: boolean;
  downloadCount: number;
  score: number;
}

export interface SubtitlesStatus {
  tracks: SubtitleTrack[];
  missing: string[];
  file: MediaFileInfo | null;
}

export interface OpenSubtitlesConfig {
  apiKey: string;
  username: string;
  password: string;
}

export interface SubtitlesConfig {
  enabled: boolean;
  autoDownload: boolean;
  languages: string[];
  preferHearingImpaired: boolean;
  includeForced: boolean;
  openSubtitles: OpenSubtitlesConfig;
}

export interface RatingValue {
  votes: number;
  value: number;
  type?: string;
}

export interface MovieRatings {
  tmdb?: RatingValue;
  imdb?: RatingValue;
  rottenTomatoes?: RatingValue;
  metacritic?: RatingValue;
  trakt?: RatingValue;
}

export interface MovieCollection {
  tmdbId: number;
  name: string;
}

export interface MovieAlternateTitle {
  title: string;
  type?: string;
  language?: string;
}

export interface MoviePersonCredit {
  name: string;
  order: number;
  character?: string;
  job?: string;
  department?: string;
  tmdbId: number;
  images: MediaImage[];
}

export interface MovieCredits {
  cast: MoviePersonCredit[];
  crew: MoviePersonCredit[];
}

export interface MovieExtraFile {
  relativePath: string;
  path: string;
  extension: string;
  type: string;
  size: number;
}

export interface MovieFilesResponse {
  media: MediaFileInfo[];
  extra: MovieExtraFile[];
}

export interface Movie {
  id: number;
  title: string;
  originalTitle: string;
  originalLanguage: string;
  sortTitle: string;
  status: string;
  overview: string;
  inCinemas?: string;
  physicalRelease?: string;
  digitalRelease?: string;
  images: MediaImage[];
  website: string;
  year: number;
  hasFile: boolean;
  movieFileId: number;
  path: string;
  qualityProfileId: number;
  monitored: boolean;
  minimumAvailability: string;
  isAvailable: boolean;
  folderName: string;
  runtime: number;
  cleanTitle: string;
  imdbId: string;
  tmdbId: number;
  titleSlug: string;
  rootFolderPath: string;
  studio: string;
  certification: string;
  youTubeTrailerId: string;
  genres: string[];
  tags: number[];
  collection?: MovieCollection | null;
  ratings: MovieRatings;
  credits: MovieCredits;
  alternateTitles: MovieAlternateTitle[];
  added: string;
  sizeOnDisk: number;
  statistics?: Record<string, unknown>;
}

export interface ConfigField {
  name: string;
  label: string;
  type: string;
  value: unknown;
  helpText?: string;
}

export interface Indexer {
  id: number;
  name: string;
  implementation: string;
  implementationName: string;
  configContract: string;
  protocol: 'torrent' | 'usenet' | string;
  privacy: 'public' | 'semiPrivate' | 'private' | string;
  enable: boolean;
  enableRss: boolean;
  enableAutomaticSearch: boolean;
  enableInteractiveSearch: boolean;
  supportsRss: boolean;
  supportsSearch: boolean;
  priority: number;
  baseUrl: string;
  apiKey?: string;
  definitionFile?: string;
  pluginId?: string;
  categories: number[];
  fields: ConfigField[];
  tags: number[];
  added: string;
}

export interface CardigannDefinition {
  id: string;
  name: string;
  description: string;
  language: string;
  type: string; // public, private, semi-private
  encoding?: string;
  protocol?: string;
  links: string[];
  fileName?: string;
  /** Raw Cardigann caps (categorymappings, modes, …) from the YAML. */
  caps?: {
    categorymappings?: Array<Record<string, unknown>>;
    modes?: Record<string, string[]>;
    allowrawsearch?: boolean;
    [key: string]: unknown;
  };
  categories?: number[];
  settings?: Array<{
    name: string;
    type: string;
    label: string;
    default?: unknown;
  }>;
}

export interface ReleaseResult {
  guid: string;
  title: string;
  indexerId: number;
  indexer: string;
  protocol: 'torrent' | 'usenet' | string;
  size: number;
  age: number;
  publishDate: string;
  downloadUrl: string;
  infoUrl: string;
  seeders: number;
  leechers: number;
  peers: number;
  categories: number[];
  quality: string;
  tvdbId?: number;
  tmdbId?: number;
  imdbId?: string;
}

export interface DownloadClient {
  id: number;
  name: string;
  implementation: string;
  implementationName: string;
  configContract: string;
  protocol: 'torrent' | 'usenet' | string;
  enable: boolean;
  priority: number;
  baseUrl: string;
  username: string;
  password?: string;
  category: string;
  removeCompletedDownloads: boolean;
  removeFailedDownloads: boolean;
  fields: ConfigField[];
  tags: number[];
}

/** Nearby client found by SSDP + HTTP fingerprint discovery. */
export interface DiscoveredDownloadClient {
  implementation: string;
  protocol: string;
  baseUrl: string;
  nameHint: string;
  detail?: string;
}

export interface Artist {
  id: number;
  title: string;
  sortTitle: string;
  status: string;
  overview: string;
  images: MediaImage[];
  path: string;
  qualityProfileId: number;
  monitored: boolean;
  foreignArtistId: string;
  titleSlug: string;
  cleanTitle: string;
  rootFolderPath: string;
  genres: string[];
  tags: number[];
  disambiguation?: string;
  added: string;
  statistics?: Record<string, unknown>;
}

export interface Album {
  id: number;
  artistId: number;
  title: string;
  foreignAlbumId: string;
  releaseDate: string;
  albumType: string;
  overview: string;
  images: MediaImage[];
  monitored: boolean;
  hasFile: boolean;
  path: string;
  year: number;
  genres: string[];
}

export interface Track {
  id: number;
  artistId: number;
  albumId: number;
  foreignTrackId: string;
  trackNumber: number;
  absoluteTrackNumber: number;
  title: string;
  durationMs: number;
  explicit: boolean;
  hasFile: boolean;
  monitored: boolean;
  mediumNumber: number;
}

export interface Author {
  id: number;
  title: string;
  sortTitle: string;
  status: string;
  overview: string;
  images: MediaImage[];
  path: string;
  qualityProfileId: number;
  monitored: boolean;
  foreignAuthorId: string;
  titleSlug: string;
  cleanTitle: string;
  rootFolderPath: string;
  genres: string[];
  tags: number[];
  added: string;
  statistics?: Record<string, unknown>;
}

export interface Book {
  id: number;
  authorId: number;
  title: string;
  foreignBookId: string;
  titleSlug: string;
  releaseDate: string;
  overview: string;
  images: MediaImage[];
  monitored: boolean;
  seriesTitle?: string;
  seriesPosition?: string;
  genres: string[];
  pageCount: number;
}

export interface Edition {
  id: number;
  bookId: number;
  authorId: number;
  foreignEditionId: string;
  title: string;
  format: 'ebook' | 'audiobook' | string;
  isbn13: string;
  isbn10: string;
  asin: string;
  language: string;
  pageCount: number;
  monitored: boolean;
  hasFile: boolean;
  path: string;
  qualityProfileId: number;
  rootFolderPath: string;
  images: MediaImage[];
  publisher: string;
  releaseDate: string;
}

export interface QueueItem {
  id: number;
  mediaType: 'tv' | 'movie' | 'music' | 'ebook' | 'audiobook' | string;
  seriesId?: number;
  episodeId?: number;
  movieId?: number;
  artistId?: number;
  albumId?: number;
  authorId?: number;
  bookId?: number;
  editionId?: number;
  title: string;
  status: string;
  trackedDownloadStatus: string;
  trackedDownloadState: string;
  size: number;
  sizeleft: number;
  timeleft: string;
  estimatedCompletionTime: string;
  protocol: 'torrent' | 'usenet' | string;
  downloadClient: string;
  downloadId: string;
  indexer: string;
  outputPath: string;
  added: string;
}

export interface QualityItem {
  id: number;
  name: string;
  source: string;
  resolution: number;
  allowed: boolean;
}

export interface QualityProfile {
  id: number;
  name: string;
  mediaType?: string; // video | music | ebook | audiobook
  upgradeAllowed: boolean;
  cutoff: number;
  items: QualityItem[];
  minFormatScore: number;
  cutoffFormatScore: number;
}

export interface RootFolder {
  id: number;
  path: string;
  mediaType: string;
  accessible: boolean;
  freeSpace: number;
  totalSpace: number;
  unmappedFolders?: unknown[];
}

export interface ConfigFieldSchema {
  name: string;
  label: string;
  type: string; // text, password, number, checkbox, select
  default?: unknown;
  required: boolean;
  helpText?: string;
}

export interface InboundWebhookSpec {
  slug?: string;
  pathAliases?: string[];
  secretSetting: string;
  description?: string;
}

export interface PluginManifest {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  types: string[];
  configSchema: ConfigFieldSchema[];
  inboundWebhooks?: InboundWebhookSpec[];
  disabledByDefault?: boolean;
}

export interface DiscoveredPlugin {
  manifest: PluginManifest;
  binaryPath: string;
  enabled: boolean;
  status: string;
  builtin?: boolean;
  lastError?: string;
  settings: Record<string, unknown>;
}

export interface CatalogPluginAsset {
  goos: string;
  goarch: string;
  url: string;
  sha256: string;
  size: number;
}

export interface CatalogPlugin {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  types: string[];
  homepage?: string;
  repository?: string;
  minReplacearrVersion?: string;
  assets: CatalogPluginAsset[];
  installed: boolean;
  installedVersion?: string;
  updateAvailable: boolean;
  hasPlatformAsset: boolean;
  source?: string;
  repoUrl?: string;
  manifestUrl?: string;
}

export interface CatalogResolveResponse {
  plugin: CatalogPlugin;
  repoUrl: string;
  manifestUrl: string;
  hasPlatformAsset: boolean;
}

export interface PluginCatalogResponse {
  catalogUrl: string;
  plugins: CatalogPlugin[];
}

export interface OverviewResponse {
  counts: {
    series: number;
    episodes: number;
    movies: number;
    artists?: number;
    albums?: number;
    authors?: number;
    books?: number;
    editions?: number;
    indexers: number;
    indexerProxies?: number;
    activeDownloads: number;
    plugins: number;
    cardigannDefinitions: number;
  };
  system: {
    version: string;
    sqliteWal: boolean;
    uptime: string;
    flareSolverrUrl?: string;
    compatEndpoints: string[] | Record<string, string>;
  };
  recentQueue: QueueItem[];
  recentHistory: HistoryRecord[];
}

export interface IndexerProxy {
  id: number;
  name: string;
  implementation: 'FlareSolverr' | 'Http' | 'Socks4' | 'Socks5' | string;
  implementationName: string;
  configContract: string;
  host: string;
  requestTimeout: number;
  sessionTtlMinutes: number;
  disableMedia: boolean;
  enable: boolean;
  fields: ConfigField[];
  tags: number[];
  added: string;
}

export interface FlareSolverrHealth {
  host: string;
  healthy: boolean;
  version?: string;
  userAgent?: string;
  message: string;
  latencyMs: number;
  sessions?: string[];
}

export interface DomainClearance {
  domain: string;
  userAgent: string;
  cookies: Array<{
    name: string;
    value: string;
    domain?: string;
    path?: string;
    httpOnly?: boolean;
    secure?: boolean;
  }>;
  updatedAt: string;
  expiresAt: string;
}

export interface Tag {
  id: number;
  label: string;
}

export interface TagDetails {
  id: number;
  label: string;
  seriesIds: number[];
  movieIds: number[];
  indexerIds: number[];
  downloadClientIds: number[];
  delayProfileIds: number[];
  releaseProfileIds: number[];
  notificationIds: number[];
  importListIds: number[];
}

export interface QualityDefinition {
  id: number;
  name: string;
  title: string;
  source: string;
  resolution: number;
  minSize: number;
  preferredSize: number;
  maxSize: number;
}

export interface HostConfig {
  bindAddress: string;
  port: number;
  urlBase: string;
  instanceName: string;
  applicationUrl: string;
  enableSsl: boolean;
  sslPort: number;
  sslCertPath: string;
  sslKeyPath: string;
  sslCertPassword: string;
  bindAddressEnvLocked: boolean;
  portEnvLocked: boolean;
  urlBaseEnvLocked: boolean;
  pendingRestart: boolean;
}

export interface SecurityConfig {
  authenticationMethod: string;
  authenticationRequired: string;
  certificateValidation: string;
  apiKey: string;
}

export interface ProxyConfig {
  enabled: boolean;
  type: string;
  hostname: string;
  port: number;
  username: string;
  password: string;
  bypassFilter: string;
  bypassLocalAddresses: boolean;
}

export interface LoggingConfig {
  logLevel: string;
  consoleLogLevel: string;
  logSizeLimit: number;
  historyCleanupDays: number;
  logIndexerResponse: boolean;
}

export interface UpdatesConfig {
  branch: string;
  updateAutomatically: boolean;
  mechanism: string;
  updateScriptPath: string;
}

export interface BackupConfig {
  folder: string;
  interval: number;
  retention: number;
}

export interface UiConfig {
  firstDayOfWeek: number;
  calendarWeekColumnHeader: string;
  shortDateFormat: string;
  longDateFormat: string;
  timeFormat: string;
  showRelativeDates: boolean;
  theme: string;
  enableColorImpairedMode: boolean;
  uiLanguage: string;
}

export interface MediaManagementConfig {
  renameEpisodes: boolean;
  renameMovies: boolean;
  replaceIllegalCharacters: boolean;
  colonReplacementFormat: string;
  customColonReplacementFormat: string;
  multiEpisodeStyle: string;
  standardEpisodeFormat: string;
  dailyEpisodeFormat: string;
  animeEpisodeFormat: string;
  seriesFolderFormat: string;
  seasonFolderFormat: string;
  specialsFolderFormat: string;
  movieFolderFormat: string;
  movieFileFormat: string;
  createEmptySeriesFolders: boolean;
  createEmptyMovieFolders: boolean;
  deleteEmptyFolders: boolean;
  episodeTitleRequired: string;
  skipFreeSpaceCheckWhenImporting: boolean;
  minimumFreeSpaceWhenImporting: number;
  copyUsingHardlinks: boolean;
  useScriptImport: boolean;
  scriptImportPath: string;
  importExtraFiles: boolean;
  extraFileExtensions: string;
  unmonitorDeletedEpisodes: boolean;
  unmonitorDeletedMovies: boolean;
  downloadPropersAndRepacks: string;
  analyzeVideoFiles: boolean;
  rescanAfterRefresh: string;
  fileDate: string;
  recycleBin: string;
  recycleBinCleanupDays: number;
  setPermissionsLinux: boolean;
  chmodFolder: string;
  chownGroup: string;
  autoRenameFolders: boolean;
  unwantedFileExtensions: string;
  tvUnwantedFileExtensions: string;
  movieUnwantedFileExtensions: string;
  musicUnwantedFileExtensions: string;
  bookUnwantedFileExtensions: string;
  deleteUnwantedFiles: boolean;
}

export interface IndexerOptions {
  minimumAge: number;
  retention: number;
  maximumSize: number;
  rssSyncInterval: number;
}

export interface DownloadClientOptions {
  enableCompletedDownloadHandling: boolean;
  autoRedownloadFailed: boolean;
  autoRedownloadFailedFromInteractiveSearch: boolean;
  deleteTorrentAndDataWhenComplete: boolean;
  deleteWhenSeedingComplete: boolean;
  downloadClientWorkingFolders: string;
  discoverSubnets: string;
  hostProcNet: string;
}

export interface PluginsConfig {
  catalogUrl: string;
}

export interface DelayProfile {
  id: number;
  enableUsenet: boolean;
  enableTorrent: boolean;
  preferredProtocol: string;
  usenetDelay: number;
  torrentDelay: number;
  bypassIfHighestQuality: boolean;
  bypassIfAboveCustomFormatScore: boolean;
  minimumCustomFormatScore: number;
  order: number;
  tags: number[];
}

export interface ReleaseProfile {
  id: number;
  enabled: boolean;
  name: string;
  required: string[];
  ignored: string[];
  indexerId: number;
  tags: number[];
}

export interface NotificationConfig {
  id: number;
  name: string;
  implementation: string;
  enable: boolean;
  onGrab: boolean;
  onImport: boolean;
  onUpgrade: boolean;
  onRename: boolean;
  onSeriesDelete: boolean;
  onMovieDelete: boolean;
  onHealthIssue: boolean;
  onHealthRestored: boolean;
  onApplicationUpdate: boolean;
  includeHealthWarnings: boolean;
  webhookUrl: string;
  pluginId: string;
  tags: number[];
}

export interface ImportListConfig {
  id: number;
  name: string;
  implementation: string;
  enable: boolean;
  qualityProfileId: number;
  rootFolderPath: string;
  monitor: string;
  searchOnAdd: boolean;
  url: string;
  tags: number[];
}

export interface ImportListOptions {
  listSyncLevel: string;
  listSyncTag: number;
}

export interface MetadataConfig {
  preferredLanguage: string;
  cleanupMetadataImages: boolean;
}

export interface MetadataConsumer {
  id: number;
  name: string;
  implementation: string;
  enable: boolean;
  seriesImages: boolean;
  seasonImages: boolean;
  episodeImages: boolean;
  movieImages: boolean;
  writeNfo: boolean;
}

export interface RemotePathMapping {
  id: number;
  host: string;
  remotePath: string;
  localPath: string;
}

export interface AppConfig {
  setupComplete: boolean;
  host: HostConfig;
  security: SecurityConfig;
  proxy: ProxyConfig;
  logging: LoggingConfig;
  updates: UpdatesConfig;
  backup: BackupConfig;
  ui: UiConfig;
  mediaManagement: MediaManagementConfig;
  indexerOptions: IndexerOptions;
  downloadClientOptions: DownloadClientOptions;
  qualityDefinitions: QualityDefinition[];
  delayProfiles: DelayProfile[];
  releaseProfiles: ReleaseProfile[];
  notifications: NotificationConfig[];
  importLists: ImportListConfig[];
  importListOptions: ImportListOptions;
  metadata: MetadataConfig;
  metadataConsumers: MetadataConsumer[];
  remotePathMappings: RemotePathMapping[];
  subtitles: SubtitlesConfig;
  plugins: PluginsConfig;
}

