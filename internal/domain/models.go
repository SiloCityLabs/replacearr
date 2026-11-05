package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Permissions constants for RBAC
const (
	PermTVRead         = "tv:read"
	PermTVWrite        = "tv:write"
	PermTVDelete       = "tv:delete"
	PermMoviesRead     = "movies:read"
	PermMoviesWrite    = "movies:write"
	PermMoviesDelete   = "movies:delete"
	PermMusicRead      = "music:read"
	PermMusicWrite     = "music:write"
	PermMusicDelete    = "music:delete"
	PermBooksRead      = "books:read"
	PermBooksWrite     = "books:write"
	PermBooksDelete    = "books:delete"
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
	PermMusicRead, PermMusicWrite, PermMusicDelete,
	PermBooksRead, PermBooksWrite, PermBooksDelete,
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
	Scope       string     `json:"scope"` // "all", "sonarr", "radarr", "prowlarr", "lidarr", "readarr"
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
	MediaType       string `json:"mediaType"` // "tv", "movie", "music", "ebook", "audiobook", "all"
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
	MediaType         string        `json:"mediaType"` // video, music, ebook, audiobook
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

// SeriesAlias is an alternate title used when building TV search queries.
type SeriesAlias struct {
	ID           int64  `json:"id"`
	SeriesID     int64  `json:"seriesId"`
	Title        string `json:"title"`
	SeasonNumber *int   `json:"seasonNumber,omitempty"`
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

// EpisodeFile is the on-disk video for an episode (native library record).
type EpisodeFile struct {
	ID           int64     `json:"id"`
	SeriesID     int64     `json:"seriesId"`
	EpisodeID    int64     `json:"episodeId"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relativePath"`
	Size         int64     `json:"size"`
	SceneName    string    `json:"sceneName"`
	ReleaseGroup string    `json:"releaseGroup"`
	Quality      string    `json:"quality"`
	Resolution   int       `json:"resolution"`
	VideoCodec   string    `json:"videoCodec"`
	AudioCodec   string    `json:"audioCodec"`
	Languages    string    `json:"languages"`
	Indexer      string    `json:"indexer"`
	DateAdded    time.Time `json:"dateAdded"`
}

// MovieFile is the on-disk video for a movie (native library record).
type MovieFile struct {
	ID           int64     `json:"id"`
	MovieID      int64     `json:"movieId"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relativePath"`
	Size         int64     `json:"size"`
	SceneName    string    `json:"sceneName"`
	ReleaseGroup string    `json:"releaseGroup"`
	Quality      string    `json:"quality"`
	Resolution   int       `json:"resolution"`
	VideoCodec   string    `json:"videoCodec"`
	AudioCodec   string    `json:"audioCodec"`
	Languages    string    `json:"languages"`
	Indexer      string    `json:"indexer"`
	DateAdded    time.Time `json:"dateAdded"`
}

// MovieFileUpdate is the editable metadata patch for a movie file.
type MovieFileUpdate struct {
	Quality      string `json:"quality"`
	Languages    string `json:"languages"`
	Indexer      string `json:"indexer"`
	ReleaseGroup string `json:"releaseGroup"`
	SceneName    string `json:"sceneName"`
	VideoCodec   string `json:"videoCodec"`
	AudioCodec   string `json:"audioCodec"`
	Resolution   int    `json:"resolution"`
}

// FileDetailProperty is a labeled media/file attribute for the details modal.
type FileDetailProperty struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FileDetails is a probe/stat summary for media or sidecar files.
type FileDetails struct {
	Path       string               `json:"path"`
	Kind       string               `json:"kind"` // media | extra
	Source     string               `json:"source,omitempty"`
	Properties []FileDetailProperty `json:"properties"`
}

// SubtitleProfile describes wanted subtitle languages and preferences.
type SubtitleProfile struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	Languages              []string `json:"languages"`
	PreferHearingImpaired  bool     `json:"preferHearingImpaired"`
	IncludeForced          bool     `json:"includeForced"`
	MinScore               int      `json:"minScore"`
	Enabled                bool     `json:"enabled"`
}

// SubtitleTrack is a downloaded (or discovered) sidecar subtitle.
type SubtitleTrack struct {
	ID               int64     `json:"id"`
	MediaType        string    `json:"mediaType"` // movie | tv
	MovieID          int64     `json:"movieId,omitempty"`
	EpisodeID        int64     `json:"episodeId,omitempty"`
	EpisodeFileID    int64     `json:"episodeFileId,omitempty"`
	MovieFileID      int64     `json:"movieFileId,omitempty"`
	Language         string    `json:"language"`
	Path             string    `json:"path"`
	Provider         string    `json:"provider"`
	ReleaseName      string    `json:"releaseName"`
	Score            int       `json:"score"`
	Forced           bool      `json:"forced"`
	HearingImpaired  bool      `json:"hearingImpaired"`
	DownloadedAt     time.Time `json:"downloadedAt"`
}

// SubtitleCandidate is a provider search hit before download.
type SubtitleCandidate struct {
	Provider        string `json:"provider"`
	FileID          int64  `json:"fileId"`
	Language        string `json:"language"`
	ReleaseName     string `json:"releaseName"`
	HearingImpaired bool   `json:"hearingImpaired"`
	Forced          bool   `json:"forced"`
	HashMatch       bool   `json:"hashMatch"`
	DownloadCount   int    `json:"downloadCount"`
	Score           int    `json:"score"`
}

// RatingValue is a single ratings provider score.
type RatingValue struct {
	Votes int     `json:"votes"`
	Value float64 `json:"value"`
	Type  string  `json:"type,omitempty"`
}

// MovieRatings holds IMDb / TMDB / RT / Metacritic / Trakt scores.
type MovieRatings struct {
	Tmdb           *RatingValue `json:"tmdb,omitempty"`
	Imdb           *RatingValue `json:"imdb,omitempty"`
	RottenTomatoes *RatingValue `json:"rottenTomatoes,omitempty"`
	Metacritic     *RatingValue `json:"metacritic,omitempty"`
	Trakt          *RatingValue `json:"trakt,omitempty"`
}

// MovieCollection is a TMDB franchise/collection membership.
type MovieCollection struct {
	TmdbID int    `json:"tmdbId"`
	Name   string `json:"name"`
}

// MovieAlternateTitle is an alternate / translated title.
type MovieAlternateTitle struct {
	Title    string `json:"title"`
	Type     string `json:"type,omitempty"`
	Language string `json:"language,omitempty"`
}

// MoviePersonCredit is a cast or crew member.
type MoviePersonCredit struct {
	Name       string       `json:"name"`
	Order      int          `json:"order"`
	Character  string       `json:"character,omitempty"`
	Job        string       `json:"job,omitempty"`
	Department string       `json:"department,omitempty"`
	TmdbID     int          `json:"tmdbId"`
	Images     []MediaImage `json:"images"`
}

// MovieCredits groups cast and crew for a movie.
type MovieCredits struct {
	Cast []MoviePersonCredit `json:"cast"`
	Crew []MoviePersonCredit `json:"crew"`
}

// MovieExtraFile is a non-video file next to the movie (subtitle, image, nfo, …).
type MovieExtraFile struct {
	RelativePath string `json:"relativePath"`
	Path         string `json:"path"`
	Extension    string `json:"extension"`
	Type         string `json:"type"` // subtitle | image | nfo | other
	Size         int64  `json:"size"`
}

// Radarr Movie Domain Models

type Movie struct {
	ID                  int64                  `json:"id"`
	Title               string                 `json:"title"`
	OriginalTitle       string                 `json:"originalTitle"`
	OriginalLanguage    string                 `json:"originalLanguage"`
	SortTitle           string                 `json:"sortTitle"`
	Status              string                 `json:"status"` // released, inCinemas, announced
	Overview            string                 `json:"overview"`
	InCinemas           string                 `json:"inCinemas,omitempty"`
	PhysicalRelease     string                 `json:"physicalRelease,omitempty"`
	DigitalRelease      string                 `json:"digitalRelease,omitempty"`
	Images              []MediaImage           `json:"images"`
	Website             string                 `json:"website"`
	Year                int                    `json:"year"`
	HasFile             bool                   `json:"hasFile"`
	MovieFileID         int64                  `json:"movieFileId"`
	Path                string                 `json:"path"`
	QualityProfileID    int64                  `json:"qualityProfileId"`
	Monitored           bool                   `json:"monitored"`
	MinimumAvailability string                 `json:"minimumAvailability"` // announced, inCinemas, released
	IsAvailable         bool                   `json:"isAvailable"`
	FolderName          string                 `json:"folderName"`
	Runtime             int                    `json:"runtime"`
	CleanTitle          string                 `json:"cleanTitle"`
	ImdbID              string                 `json:"imdbId"`
	TmdbID              int                    `json:"tmdbId"`
	TitleSlug           string                 `json:"titleSlug"`
	RootFolderPath      string                 `json:"rootFolderPath"`
	Studio              string                 `json:"studio"`
	Certification       string                 `json:"certification"`
	YouTubeTrailerID    string                 `json:"youTubeTrailerId"`
	Genres              []string               `json:"genres"`
	Tags                []int64                `json:"tags"`
	Collection          *MovieCollection       `json:"collection,omitempty"`
	Ratings             MovieRatings           `json:"ratings"`
	Credits             MovieCredits           `json:"credits"`
	AlternateTitles     []MovieAlternateTitle  `json:"alternateTitles"`
	Added               time.Time              `json:"added"`
	SizeOnDisk          int64                  `json:"sizeOnDisk"`
	Statistics          map[string]any         `json:"statistics,omitempty"`
}

// Music (Lidarr-like) Domain Models

type Artist struct {
	ID               int64          `json:"id"`
	Title            string         `json:"title"`
	SortTitle        string         `json:"sortTitle"`
	Status           string         `json:"status"`
	Overview         string         `json:"overview"`
	Images           []MediaImage   `json:"images"`
	Path             string         `json:"path"`
	QualityProfileID int64          `json:"qualityProfileId"`
	Monitored        bool           `json:"monitored"`
	ForeignArtistID  string         `json:"foreignArtistId"` // MusicBrainz artist MBID
	TitleSlug        string         `json:"titleSlug"`
	CleanTitle       string         `json:"cleanTitle"`
	RootFolderPath   string         `json:"rootFolderPath"`
	Genres           []string       `json:"genres"`
	Tags             []int64        `json:"tags"`
	Disambiguation   string         `json:"disambiguation"`
	Added            time.Time      `json:"added"`
	Statistics       map[string]any `json:"statistics,omitempty"`
}

type Album struct {
	ID             int64        `json:"id"`
	ArtistID       int64        `json:"artistId"`
	Title          string       `json:"title"`
	ForeignAlbumID string       `json:"foreignAlbumId"` // MusicBrainz release-group MBID
	ReleaseDate    string       `json:"releaseDate"`
	AlbumType      string       `json:"albumType"` // Album, EP, Single, ...
	Overview       string       `json:"overview"`
	Images         []MediaImage `json:"images"`
	Monitored      bool         `json:"monitored"`
	HasFile        bool         `json:"hasFile"`
	Path           string       `json:"path"`
	Year           int          `json:"year"`
	Genres         []string     `json:"genres"`
}

type Track struct {
	ID                   int64  `json:"id"`
	ArtistID             int64  `json:"artistId"`
	AlbumID              int64  `json:"albumId"`
	ForeignTrackID       string `json:"foreignTrackId"`
	TrackNumber          int    `json:"trackNumber"`
	AbsoluteTrackNumber  int    `json:"absoluteTrackNumber"`
	Title                string `json:"title"`
	DurationMs           int    `json:"durationMs"`
	Explicit             bool   `json:"explicit"`
	HasFile              bool   `json:"hasFile"`
	Monitored            bool   `json:"monitored"`
	MediumNumber         int    `json:"mediumNumber"`
}

// AlbumWithTracks pairs an album with its track list for MusicBrainz sync.
type AlbumWithTracks struct {
	Album  Album   `json:"album"`
	Tracks []Track `json:"tracks"`
}

// Books (ebook + audiobook) Domain Models

type Author struct {
	ID               int64          `json:"id"`
	Title            string         `json:"title"`
	SortTitle        string         `json:"sortTitle"`
	Status           string         `json:"status"`
	Overview         string         `json:"overview"`
	Images           []MediaImage   `json:"images"`
	Path             string         `json:"path"`
	QualityProfileID int64          `json:"qualityProfileId"`
	Monitored        bool           `json:"monitored"`
	ForeignAuthorID  string         `json:"foreignAuthorId"` // OpenLibrary author key
	TitleSlug        string         `json:"titleSlug"`
	CleanTitle       string         `json:"cleanTitle"`
	RootFolderPath   string         `json:"rootFolderPath"`
	Genres           []string       `json:"genres"`
	Tags             []int64        `json:"tags"`
	Added            time.Time      `json:"added"`
	Statistics       map[string]any `json:"statistics,omitempty"`
}

type Book struct {
	ID            int64        `json:"id"`
	AuthorID      int64        `json:"authorId"`
	Title         string       `json:"title"`
	ForeignBookID string       `json:"foreignBookId"` // OpenLibrary work key
	TitleSlug     string       `json:"titleSlug"`
	ReleaseDate   string       `json:"releaseDate"`
	Overview      string       `json:"overview"`
	Images        []MediaImage `json:"images"`
	Monitored     bool         `json:"monitored"`
	SeriesTitle   string       `json:"seriesTitle"`
	SeriesPosition string      `json:"seriesPosition"`
	Genres        []string     `json:"genres"`
	PageCount     int          `json:"pageCount"`
}

type Edition struct {
	ID               int64        `json:"id"`
	BookID           int64        `json:"bookId"`
	AuthorID         int64        `json:"authorId"`
	ForeignEditionID string       `json:"foreignEditionId"`
	Title            string       `json:"title"`
	Format           string       `json:"format"` // ebook | audiobook
	Isbn13           string       `json:"isbn13"`
	Isbn10           string       `json:"isbn10"`
	Asin             string       `json:"asin"`
	Language         string       `json:"language"`
	PageCount        int          `json:"pageCount"`
	Monitored        bool         `json:"monitored"`
	HasFile          bool         `json:"hasFile"`
	Path             string       `json:"path"`
	QualityProfileID int64        `json:"qualityProfileId"`
	RootFolderPath   string       `json:"rootFolderPath"`
	Images           []MediaImage `json:"images"`
	Publisher        string       `json:"publisher"`
	ReleaseDate      string       `json:"releaseDate"`
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
	BaseURL                  string        `json:"baseUrl"` // e.g. http://localhost:8080 or https://qbittorrent.example/qbittorrent
	Username                 string        `json:"username"`
	Password                 string        `json:"password,omitempty"`
	Category                 string        `json:"category"`
	RemoveCompletedDownloads bool          `json:"removeCompletedDownloads"`
	RemoveFailedDownloads    bool          `json:"removeFailedDownloads"`
	Fields                   []ConfigField `json:"fields"`
	Tags                     []int64       `json:"tags"`
}

// ProtocolForDownloadClientImplementation returns the protocol fixed by client type.
// Built-in torrent clients are always "torrent"; SABnzbd/NZBGet are always "usenet".
// Plugin (and unknown) implementations keep preferred when it is torrent/usenet.
func ProtocolForDownloadClientImplementation(implementation, preferred string) string {
	switch strings.TrimSpace(implementation) {
	case "SABnzbd", "NZBGet":
		return "usenet"
	case "qBittorrent", "Transmission", "Deluge":
		return "torrent"
	case "Plugin":
		if strings.EqualFold(preferred, "usenet") {
			return "usenet"
		}
		return "torrent"
	default:
		return "torrent"
	}
}

// NormalizeDownloadClientBaseURL accepts a full client URL (scheme, host, optional
// port and path) and returns a trimmed canonical form. Empty input defaults to
// http://localhost:8080. A missing scheme is treated as http.
func NormalizeDownloadClientBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "http://localhost:8080", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("base URL must use http or https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", errors.New("base URL must include a host")
	}
	path := u.EscapedPath()
	if path == "/" {
		path = ""
	}
	out := (&url.URL{Scheme: scheme, Host: u.Host, Path: path}).String()
	return strings.TrimRight(out, "/"), nil
}

// SplitDownloadClientBaseURL derives legacy host/port/ssl values for columns kept
// after the base_url migration.
func SplitDownloadClientBaseURL(base string) (host string, port int, useSSL bool) {
	normalized, err := NormalizeDownloadClientBaseURL(base)
	if err != nil {
		return "localhost", 8080, false
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return "localhost", 8080, false
	}
	useSSL = strings.EqualFold(u.Scheme, "https")
	host = u.Hostname()
	if host == "" {
		host = "localhost"
	}
	if p := u.Port(); p != "" {
		port, _ = strconv.Atoi(p)
	}
	if port <= 0 {
		if useSSL {
			port = 443
		} else {
			port = 80
		}
	}
	return host, port, useSSL
}

type QueueItem struct {
	ID                    int64     `json:"id"`
	MediaType             string    `json:"mediaType"` // tv, movie, music, ebook, audiobook
	SeriesID              int64     `json:"seriesId,omitempty"`
	EpisodeID             int64     `json:"episodeId,omitempty"`
	MovieID               int64     `json:"movieId,omitempty"`
	ArtistID              int64     `json:"artistId,omitempty"`
	AlbumID               int64     `json:"albumId,omitempty"`
	AuthorID              int64     `json:"authorId,omitempty"`
	BookID                int64     `json:"bookId,omitempty"`
	EditionID             int64     `json:"editionId,omitempty"`
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
	ArtistID    int64          `json:"artistId,omitempty"`
	AlbumID     int64          `json:"albumId,omitempty"`
	AuthorID    int64          `json:"authorId,omitempty"`
	BookID      int64          `json:"bookId,omitempty"`
	EditionID   int64          `json:"editionId,omitempty"`
	SourceTitle string         `json:"sourceTitle"`
	Quality     map[string]any `json:"quality"`
	Date        time.Time      `json:"date"`
	EventType   string         `json:"eventType"` // grabbed, downloadFolderImported, downloadFailed, blocked
	Data        map[string]any `json:"data"`
}

type BlocklistItem struct {
	ID              int64     `json:"id"`
	MediaType       string    `json:"mediaType"`
	SourceTitle     string    `json:"sourceTitle"`
	TorrentInfoHash string    `json:"torrentInfoHash,omitempty"`
	Indexer         string    `json:"indexer,omitempty"`
	Message         string    `json:"message,omitempty"`
	SeriesID        int64     `json:"seriesId,omitempty"`
	MovieID         int64     `json:"movieId,omitempty"`
	ArtistID        int64     `json:"artistId,omitempty"`
	AuthorID        int64     `json:"authorId,omitempty"`
	Size            int64     `json:"size,omitempty"`
	Protocol        string    `json:"protocol,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ImportListExclusion blocks a title from being re-added by import lists.
type ImportListExclusion struct {
	ID        int64     `json:"id"`
	MediaType string    `json:"mediaType"`
	TmdbID    int       `json:"tmdbId,omitempty"`
	TvdbID    int       `json:"tvdbId,omitempty"`
	ImdbID    string    `json:"imdbId,omitempty"`
	Title     string    `json:"title"`
	Year      int       `json:"year,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type CalendarEvent struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Date      time.Time `json:"date"`
	MediaType string    `json:"mediaType"`
	Status    string    `json:"status"` // downloaded, missing, queued, unreleased
	SeriesID  int64     `json:"seriesId,omitempty"`
	MovieID   int64     `json:"movieId,omitempty"`
	ArtistID  int64     `json:"artistId,omitempty"`
	AlbumID   int64     `json:"albumId,omitempty"`
	BookID    int64     `json:"bookId,omitempty"`
	EpisodeID int64     `json:"episodeId,omitempty"`
}

type MediaStatistics struct {
	MediaType              string            `json:"mediaType"`
	ItemCount              int               `json:"itemCount"`
	MonitoredCount         int               `json:"monitoredCount"`
	WithFileCount          int               `json:"withFileCount"`
	MissingCount           int               `json:"missingCount"`
	UnreleasedCount        int               `json:"unreleasedCount"`
	SizeOnDisk             int64             `json:"sizeOnDisk"`
	AverageFileSize        int64             `json:"averageFileSize"`
	StatusBreakdown        []NamedCount      `json:"statusBreakdown,omitempty"`
	SeriesTypeBreakdown    []NamedCount      `json:"seriesTypeBreakdown,omitempty"`
	EpisodeStatusBreakdown []NamedCount      `json:"episodeStatusBreakdown,omitempty"`
	QualityProfiles        []NamedCount      `json:"qualityProfiles,omitempty"`
	QualityBreakdown       []QualityStat     `json:"qualityBreakdown,omitempty"`
	Genres                 []NamedCount      `json:"genres,omitempty"`
	FileTypes              []NamedCount      `json:"fileTypes,omitempty"`
	MissingItems           []MissingStatItem `json:"missingItems,omitempty"`
}

// NamedCount is a labeled tally for charts (status, genre, profile, etc.).
type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type QualityStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
}

type MissingStatItem struct {
	ID     int64  `json:"id,omitempty"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

type HealthCheck struct {
	Source  string    `json:"source"`
	Type    string    `json:"type"` // ok, warning, error
	Message string    `json:"message"`
	Time    time.Time `json:"time,omitempty"`
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
