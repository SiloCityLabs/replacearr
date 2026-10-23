package domain

import "time"

// Permissions constants for RBAC
const (
	PermTVRead         = "tv:read"
	PermTVWrite        = "tv:write"
	PermTVDelete       = "tv:delete"
	PermMoviesRead     = "movies:read"
	PermMoviesWrite    = "movies:write"
	PermMoviesDelete   = "movies:delete"
	PermIndexersRead   = "indexers:read"
	PermIndexersWrite  = "indexers:write"
	PermIndexersSearch = "indexers:search"
	PermDownloadsRead  = "downloads:read"
	PermDownloadsWrite = "downloads:write"
	PermProfilesRead   = "profiles:read"
	PermProfilesWrite  = "profiles:write"
	PermPluginsRead    = "plugins:read"
	PermPluginsManage  = "plugins:manage"
	PermUsersRead      = "users:read"
	PermUsersManage    = "users:manage"
	PermSystemRead     = "system:read"
	PermSystemManage   = "system:manage"
	PermAdminAll       = "*"
)

var AllPermissions = []string{
	PermTVRead, PermTVWrite, PermTVDelete,
	PermMoviesRead, PermMoviesWrite, PermMoviesDelete,
	PermIndexersRead, PermIndexersWrite, PermIndexersSearch,
	PermDownloadsRead, PermDownloadsWrite,
	PermProfilesRead, PermProfilesWrite,
	PermPluginsRead, PermPluginsManage,
	PermUsersRead, PermUsersManage,
	PermSystemRead, PermSystemManage,
}

type Role struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	BuiltIn     bool      `json:"builtIn"`
	CreatedAt   time.Time `json:"createdAt"`
}

type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	RoleID       int64      `json:"roleId"`
	RoleName     string     `json:"roleName"`
	Permissions  []string   `json:"permissions"`
	IsActive     bool       `json:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type APIKey struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	KeyHash     string     `json:"-"`
	KeyPrefix   string     `json:"keyPrefix"`
	RawKey      string     `json:"rawKey,omitempty"` // Only returned on creation or for system compat key
	UserID      int64      `json:"userId"`
	Scope       string     `json:"scope"` // "all", "sonarr", "radarr", "prowlarr"
	Permissions []string   `json:"permissions"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Shared *arr Core Models

type Tag struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type RootFolder struct {
	ID              int64  `json:"id"`
	Path            string `json:"path"`
	MediaType       string `json:"mediaType"` // "tv", "movie", "all"
	Accessible      bool   `json:"accessible"`
	FreeSpace       int64  `json:"freeSpace"`
	TotalSpace      int64  `json:"totalSpace"`
	UnmappedFolders []any  `json:"unmappedFolders"`
}

type QualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	Resolution int    `json:"resolution"`
	Allowed    bool   `json:"allowed"`
}

type QualityProfile struct {
	ID                int64         `json:"id"`
	Name              string        `json:"name"`
	UpgradeAllowed    bool          `json:"upgradeAllowed"`
	Cutoff            int           `json:"cutoff"`
	Items             []QualityItem `json:"items"`
	MinFormatScore    int           `json:"minFormatScore"`
	CutoffFormatScore int           `json:"cutoffFormatScore"`
}

// Sonarr TV Domain Models

type Season struct {
	SeasonNumber int            `json:"seasonNumber"`
	Monitored    bool           `json:"monitored"`
	Statistics   map[string]any `json:"statistics,omitempty"`
}

type MediaImage struct {
	CoverType string `json:"coverType"` // poster, fanart, banner
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

type Series struct {
	ID                int64          `json:"id"`
	Title             string         `json:"title"`
	SortTitle         string         `json:"sortTitle"`
	Status            string         `json:"status"` // continuing, ended, upcoming
	Ended             bool           `json:"ended"`
	Overview          string         `json:"overview"`
	Network           string         `json:"network"`
	AirTime           string         `json:"airTime"`
	Images            []MediaImage   `json:"images"`
	Seasons           []Season       `json:"seasons"`
	Year              int            `json:"year"`
	Path              string         `json:"path"`
	QualityProfileID  int64          `json:"qualityProfileId"`
	LanguageProfileID int64          `json:"languageProfileId"`
	SeasonFolder      bool           `json:"seasonFolder"`
	Monitored         bool           `json:"monitored"`
	UseSceneNumbering bool           `json:"useSceneNumbering"`
	Runtime           int            `json:"runtime"`
	TvdbID            int            `json:"tvdbId"`
	TvRageID          int            `json:"tvRageId"`
	TvMazeID          int            `json:"tvMazeId"`
	ImdbID            string         `json:"imdbId"`
	TitleSlug         string         `json:"titleSlug"`
	RootFolderPath    string         `json:"rootFolderPath"`
	SeriesType        string         `json:"seriesType"` // standard, daily, anime
	CleanTitle        string         `json:"cleanTitle"`
	Genres            []string       `json:"genres"`
	Tags              []int64        `json:"tags"`
	Added             time.Time      `json:"added"`
	Statistics        map[string]any `json:"statistics"`
}

type Episode struct {
	ID                 int64     `json:"id"`
	SeriesID           int64     `json:"seriesId"`
	TvdbID             int       `json:"tvdbId"`
	EpisodeFileID      int64     `json:"episodeFileId"`
	SeasonNumber       int       `json:"seasonNumber"`
	EpisodeNumber      int       `json:"episodeNumber"`
	Title              string    `json:"title"`
	AirDate            string    `json:"airDate"`
	AirDateUtc         time.Time `json:"airDateUtc"`
	Overview           string    `json:"overview"`
	HasFile            bool      `json:"hasFile"`
	Monitored          bool      `json:"monitored"`
	AbsoluteEpisodeNum int       `json:"absoluteEpisodeNumber,omitempty"`
}

// Radarr Movie Domain Models

type Movie struct {
	ID                  int64          `json:"id"`
	Title               string         `json:"title"`
	OriginalTitle       string         `json:"originalTitle"`
	SortTitle           string         `json:"sortTitle"`
	Status              string         `json:"status"` // released, inCinemas, announced
	Overview            string         `json:"overview"`
	InCinemas           string         `json:"inCinemas,omitempty"`
	PhysicalRelease     string         `json:"physicalRelease,omitempty"`
	DigitalRelease      string         `json:"digitalRelease,omitempty"`
	Images              []MediaImage   `json:"images"`
	Website             string         `json:"website"`
	Year                int            `json:"year"`
	HasFile             bool           `json:"hasFile"`
	MovieFileID         int64          `json:"movieFileId"`
	Path                string         `json:"path"`
	QualityProfileID    int64          `json:"qualityProfileId"`
	Monitored           bool           `json:"monitored"`
	MinimumAvailability string         `json:"minimumAvailability"` // announced, inCinemas, released
	IsAvailable         bool           `json:"isAvailable"`
	FolderName          string         `json:"folderName"`
	Runtime             int            `json:"runtime"`
	CleanTitle          string         `json:"cleanTitle"`
	ImdbID              string         `json:"imdbId"`
	TmdbID              int            `json:"tmdbId"`
	TitleSlug           string         `json:"titleSlug"`
	RootFolderPath      string         `json:"rootFolderPath"`
	Studio              string         `json:"studio"`
	Genres              []string       `json:"genres"`
	Tags                []int64        `json:"tags"`
	Added               time.Time      `json:"added"`
	SizeOnDisk          int64          `json:"sizeOnDisk"`
	Statistics          map[string]any `json:"statistics,omitempty"`
}

// Prowlarr Indexer & Search Models

type ConfigField struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // textbox, password, number, checkbox, select
	Value    any    `json:"value"`
	HelpText string `json:"helpText,omitempty"`
}

type Indexer struct {
	ID                      int64         `json:"id"`
	Name                    string        `json:"name"`
	Implementation          string        `json:"implementation"` // Torznab, Newznab, Cardigann, Plugin
	ImplementationName      string        `json:"implementationName"`
	ConfigContract          string        `json:"configContract"`
	Protocol                string        `json:"protocol"` // torrent, usenet
	Privacy                 string        `json:"privacy"`  // public, semiPrivate, private
	Enable                  bool          `json:"enable"`
	EnableRss               bool          `json:"enableRss"`
	EnableAutomaticSearch   bool          `json:"enableAutomaticSearch"`
	EnableInteractiveSearch bool          `json:"enableInteractiveSearch"`
	SupportsRss             bool          `json:"supportsRss"`
	SupportsSearch          bool          `json:"supportsSearch"`
	Priority                int           `json:"priority"`
	BaseURL                 string        `json:"baseUrl"`
	APIKey                  string        `json:"apiKey,omitempty"`
	DefinitionFile          string        `json:"definitionFile,omitempty"`
	PluginID                string        `json:"pluginId,omitempty"`
	Categories              []int         `json:"categories"`
	Fields                  []ConfigField `json:"fields"`
	Tags                    []int64       `json:"tags"`
	Added                   time.Time     `json:"added"`
}

type ReleaseResult struct {
	Guid        string    `json:"guid"`
	Title       string    `json:"title"`
	IndexerID   int64     `json:"indexerId"`
	Indexer     string    `json:"indexer"`
	Protocol    string    `json:"protocol"` // torrent, usenet
	Size        int64     `json:"size"`
	Age         int       `json:"age"`
	PublishDate time.Time `json:"publishDate"`
	DownloadURL string    `json:"downloadUrl"`
	InfoURL     string    `json:"infoUrl"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	Peers       int       `json:"peers"`
	Categories  []int     `json:"categories"`
	Quality     string    `json:"quality"`
	TvdbID      int       `json:"tvdbId,omitempty"`
	TmdbID      int       `json:"tmdbId,omitempty"`
	ImdbID      string    `json:"imdbId,omitempty"`
}

// Download Client & Queue Models

type DownloadClient struct {
	ID                       int64         `json:"id"`
	Name                     string        `json:"name"`
	Implementation           string        `json:"implementation"` // qBittorrent, Transmission, SABnzbd, NZBGet, MockClient, Plugin
	ImplementationName       string        `json:"implementationName"`
	ConfigContract           string        `json:"configContract"`
	Protocol                 string        `json:"protocol"` // torrent, usenet
	Enable                   bool          `json:"enable"`
	Priority                 int           `json:"priority"`
	Host                     string        `json:"host"`
	Port                     int           `json:"port"`
	UseSsl                   bool          `json:"useSsl"`
	Username                 string        `json:"username"`
	Password                 string        `json:"password,omitempty"`
	Category                 string        `json:"category"`
	RemoveCompletedDownloads bool          `json:"removeCompletedDownloads"`
	RemoveFailedDownloads    bool          `json:"removeFailedDownloads"`
	Fields                   []ConfigField `json:"fields"`
	Tags                     []int64       `json:"tags"`
}

type QueueItem struct {
	ID                    int64     `json:"id"`
	MediaType             string    `json:"mediaType"` // tv, movie
	SeriesID              int64     `json:"seriesId,omitempty"`
	EpisodeID             int64     `json:"episodeId,omitempty"`
	MovieID               int64     `json:"movieId,omitempty"`
	Title                 string    `json:"title"`
	Status                string    `json:"status"`                // downloading, queued, completed, paused, warning
	TrackedDownloadStatus string    `json:"trackedDownloadStatus"` // ok, warning, error
	TrackedDownloadState  string    `json:"trackedDownloadState"`  // downloading, importPending, imported
	Size                  int64     `json:"size"`
	Sizeleft              int64     `json:"sizeleft"`
	Timeleft              string    `json:"timeleft"`
	EstimatedCompletion   time.Time `json:"estimatedCompletionTime"`
	Protocol              string    `json:"protocol"`
	DownloadClient        string    `json:"downloadClient"`
	DownloadID            string    `json:"downloadId"`
	Indexer               string    `json:"indexer"`
	OutputPath            string    `json:"outputPath"`
	Added                 time.Time `json:"added"`
}

type CommandRecord struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	CommandName string         `json:"commandName"`
	Message     string         `json:"message"`
	Body        map[string]any `json:"body"`
	Priority    string         `json:"priority"`
	Status      string         `json:"status"` // queued, started, completed, failed
	Queued      time.Time      `json:"queued"`
	Started     time.Time      `json:"started"`
	Ended       time.Time      `json:"ended"`
	Trigger     string         `json:"trigger"`
}

type HistoryRecord struct {
	ID          int64          `json:"id"`
	MediaType   string         `json:"mediaType"`
	SeriesID    int64          `json:"seriesId,omitempty"`
	EpisodeID   int64          `json:"episodeId,omitempty"`
	MovieID     int64          `json:"movieId,omitempty"`
	SourceTitle string         `json:"sourceTitle"`
	Quality     map[string]any `json:"quality"`
	Date        time.Time      `json:"date"`
	EventType   string         `json:"eventType"` // grabbed, downloadFolderImported
	Data        map[string]any `json:"data"`
}

// Indexer Proxy (FlareSolverr / HTTP / SOCKS) Models

type IndexerProxy struct {
	ID                 int64         `json:"id"`
	Name               string        `json:"name"`
	Implementation     string        `json:"implementation"`     // FlareSolverr, Http, Socks4, Socks5
	ImplementationName string        `json:"implementationName"` // FlareSolverr
	ConfigContract     string        `json:"configContract"`     // FlareSolverrSettings
	Host               string        `json:"host"`               // e.g. http://localhost:8191/
	RequestTimeout     int           `json:"requestTimeout"`     // seconds (1..180, default 60)
	SessionTTLMinutes  int           `json:"sessionTtlMinutes"`  // optional session rotation TTL in minutes
	DisableMedia       bool          `json:"disableMedia"`       // block images/css/fonts in headless Chrome
	Enable             bool          `json:"enable"`
	OnHealthIssue      bool          `json:"onHealthIssue"`
	SupportsOnHealth   bool          `json:"supportsOnHealthIssue"`
	IncludeWarnings    bool          `json:"includeHealthWarnings"`
	Fields             []ConfigField `json:"fields"`
	Tags               []int64       `json:"tags"`
	Added              time.Time     `json:"added"`
}
