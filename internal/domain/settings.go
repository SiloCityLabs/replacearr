package domain

// AppConfig is the single settings document stored under the "app" key.
type AppConfig struct {
	Host                  HostConfig            `json:"host"`
	Security              SecurityConfig        `json:"security"`
	Proxy                 ProxyConfig           `json:"proxy"`
	Logging               LoggingConfig         `json:"logging"`
	Analytics             AnalyticsConfig       `json:"analytics"`
	Updates               UpdateConfig          `json:"updates"`
	Backup                BackupConfig          `json:"backup"`
	UI                    UIConfig              `json:"ui"`
	MediaManagement       MediaManagementConfig `json:"mediaManagement"`
	IndexerOptions        IndexerOptions        `json:"indexerOptions"`
	DownloadClientOptions DownloadClientOptions `json:"downloadClientOptions"`
	QualityDefinitions    []QualityDefinition   `json:"qualityDefinitions"`
	DelayProfiles         []DelayProfile        `json:"delayProfiles"`
	ReleaseProfiles       []ReleaseProfile      `json:"releaseProfiles"`
	Notifications         []NotificationConfig  `json:"notifications"`
	ImportLists           []ImportListConfig    `json:"importLists"`
	ImportListOptions     ImportListOptions     `json:"importListOptions"`
	Metadata              MetadataConfig        `json:"metadata"`
	MetadataConsumers     []MetadataConsumer    `json:"metadataConsumers"`
	RemotePathMappings    []RemotePathMapping   `json:"remotePathMappings"`
}

type HostConfig struct {
	BindAddress          string `json:"bindAddress"`
	Port                 int    `json:"port"`
	URLBase              string `json:"urlBase"`
	InstanceName         string `json:"instanceName"`
	ApplicationURL       string `json:"applicationUrl"`
	EnableSSL            bool   `json:"enableSsl"`
	SSLPort              int    `json:"sslPort"`
	SSLCertPath          string `json:"sslCertPath"`
	SSLKeyPath           string `json:"sslKeyPath"`
	SSLCertPassword      string `json:"sslCertPassword"`
	BindAddressEnvLocked bool   `json:"bindAddressEnvLocked"`
	PortEnvLocked        bool   `json:"portEnvLocked"`
	URLBaseEnvLocked     bool   `json:"urlBaseEnvLocked"`
	PendingRestart       bool   `json:"pendingRestart"`
}

type SecurityConfig struct {
	AuthenticationMethod   string `json:"authenticationMethod"`
	AuthenticationRequired string `json:"authenticationRequired"`
	CertificateValidation  string `json:"certificateValidation"`
	APIKey                 string `json:"apiKey"`
}

type ProxyConfig struct {
	Enabled              bool   `json:"enabled"`
	Type                 string `json:"type"`
	Hostname             string `json:"hostname"`
	Port                 int    `json:"port"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	BypassFilter         string `json:"bypassFilter"`
	BypassLocalAddresses bool   `json:"bypassLocalAddresses"`
}

type LoggingConfig struct {
	LogLevel           string `json:"logLevel"`
	ConsoleLogLevel    string `json:"consoleLogLevel"`
	LogSizeLimit       int    `json:"logSizeLimit"`
	HistoryCleanupDays int    `json:"historyCleanupDays"`
	LogIndexerResponse bool   `json:"logIndexerResponse"`
}

type AnalyticsConfig struct {
	Enabled bool `json:"enabled"`
}

type UpdateConfig struct {
	Branch              string `json:"branch"`
	UpdateAutomatically bool   `json:"updateAutomatically"`
	Mechanism           string `json:"mechanism"`
	UpdateScriptPath    string `json:"updateScriptPath"`
}

type BackupConfig struct {
	Folder    string `json:"folder"`
	Interval  int    `json:"interval"`
	Retention int    `json:"retention"`
}

type UIConfig struct {
	FirstDayOfWeek           int    `json:"firstDayOfWeek"`
	CalendarWeekColumnHeader string `json:"calendarWeekColumnHeader"`
	ShortDateFormat          string `json:"shortDateFormat"`
	LongDateFormat           string `json:"longDateFormat"`
	TimeFormat               string `json:"timeFormat"`
	ShowRelativeDates        bool   `json:"showRelativeDates"`
	Theme                    string `json:"theme"`
	EnableColorImpairedMode  bool   `json:"enableColorImpairedMode"`
	UILanguage               string `json:"uiLanguage"`
}

type MediaManagementConfig struct {
	RenameEpisodes                  bool   `json:"renameEpisodes"`
	RenameMovies                    bool   `json:"renameMovies"`
	ReplaceIllegalCharacters        bool   `json:"replaceIllegalCharacters"`
	ColonReplacementFormat          string `json:"colonReplacementFormat"`
	CustomColonReplacementFormat    string `json:"customColonReplacementFormat"`
	MultiEpisodeStyle               string `json:"multiEpisodeStyle"`
	StandardEpisodeFormat           string `json:"standardEpisodeFormat"`
	DailyEpisodeFormat              string `json:"dailyEpisodeFormat"`
	AnimeEpisodeFormat              string `json:"animeEpisodeFormat"`
	SeriesFolderFormat              string `json:"seriesFolderFormat"`
	SeasonFolderFormat              string `json:"seasonFolderFormat"`
	SpecialsFolderFormat            string `json:"specialsFolderFormat"`
	MovieFolderFormat               string `json:"movieFolderFormat"`
	MovieFileFormat                 string `json:"movieFileFormat"`
	CreateEmptySeriesFolders        bool   `json:"createEmptySeriesFolders"`
	CreateEmptyMovieFolders         bool   `json:"createEmptyMovieFolders"`
	DeleteEmptyFolders              bool   `json:"deleteEmptyFolders"`
	EpisodeTitleRequired            string `json:"episodeTitleRequired"`
	SkipFreeSpaceCheckWhenImporting bool   `json:"skipFreeSpaceCheckWhenImporting"`
	MinimumFreeSpaceWhenImporting   int    `json:"minimumFreeSpaceWhenImporting"`
	CopyUsingHardlinks              bool   `json:"copyUsingHardlinks"`
	UseScriptImport                 bool   `json:"useScriptImport"`
	ScriptImportPath                string `json:"scriptImportPath"`
	ImportExtraFiles                bool   `json:"importExtraFiles"`
	ExtraFileExtensions             string `json:"extraFileExtensions"`
	UnmonitorDeletedEpisodes        bool   `json:"unmonitorDeletedEpisodes"`
	UnmonitorDeletedMovies          bool   `json:"unmonitorDeletedMovies"`
	DownloadPropersAndRepacks       string `json:"downloadPropersAndRepacks"`
	AnalyzeVideoFiles               bool   `json:"analyzeVideoFiles"`
	RescanAfterRefresh              string `json:"rescanAfterRefresh"`
	FileDate                        string `json:"fileDate"`
	RecycleBin                      string `json:"recycleBin"`
	RecycleBinCleanupDays           int    `json:"recycleBinCleanupDays"`
	SetPermissionsLinux             bool   `json:"setPermissionsLinux"`
	ChmodFolder                     string `json:"chmodFolder"`
	ChownGroup                      string `json:"chownGroup"`
	AutoRenameFolders               bool   `json:"autoRenameFolders"`
}

type IndexerOptions struct {
	MinimumAge      int `json:"minimumAge"`
	Retention       int `json:"retention"`
	MaximumSize     int `json:"maximumSize"`
	RssSyncInterval int `json:"rssSyncInterval"`
}

type DownloadClientOptions struct {
	EnableCompletedDownloadHandling           bool   `json:"enableCompletedDownloadHandling"`
	AutoRedownloadFailed                      bool   `json:"autoRedownloadFailed"`
	AutoRedownloadFailedFromInteractiveSearch bool   `json:"autoRedownloadFailedFromInteractiveSearch"`
	DownloadClientWorkingFolders              string `json:"downloadClientWorkingFolders"`
}

type QualityDefinition struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Title         string  `json:"title"`
	Source        string  `json:"source"`
	Resolution    int     `json:"resolution"`
	MinSize       float64 `json:"minSize"`
	PreferredSize float64 `json:"preferredSize"`
	MaxSize       float64 `json:"maxSize"`
}

type DelayProfile struct {
	ID                             int64   `json:"id"`
	EnableUsenet                   bool    `json:"enableUsenet"`
	EnableTorrent                  bool    `json:"enableTorrent"`
	PreferredProtocol              string  `json:"preferredProtocol"`
	UsenetDelay                    int     `json:"usenetDelay"`
	TorrentDelay                   int     `json:"torrentDelay"`
	BypassIfHighestQuality         bool    `json:"bypassIfHighestQuality"`
	BypassIfAboveCustomFormatScore bool    `json:"bypassIfAboveCustomFormatScore"`
	MinimumCustomFormatScore       int     `json:"minimumCustomFormatScore"`
	Order                          int     `json:"order"`
	Tags                           []int64 `json:"tags"`
}

type ReleaseProfile struct {
	ID        int64    `json:"id"`
	Enabled   bool     `json:"enabled"`
	Name      string   `json:"name"`
	Required  []string `json:"required"`
	Ignored   []string `json:"ignored"`
	IndexerID int64    `json:"indexerId"`
	Tags      []int64  `json:"tags"`
}

type NotificationConfig struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Implementation        string  `json:"implementation"`
	Enable                bool    `json:"enable"`
	OnGrab                bool    `json:"onGrab"`
	OnImport              bool    `json:"onImport"`
	OnUpgrade             bool    `json:"onUpgrade"`
	OnRename              bool    `json:"onRename"`
	OnSeriesDelete        bool    `json:"onSeriesDelete"`
	OnMovieDelete         bool    `json:"onMovieDelete"`
	OnHealthIssue         bool    `json:"onHealthIssue"`
	OnHealthRestored      bool    `json:"onHealthRestored"`
	OnApplicationUpdate   bool    `json:"onApplicationUpdate"`
	IncludeHealthWarnings bool    `json:"includeHealthWarnings"`
	WebhookURL            string  `json:"webhookUrl"`
	PluginID              string  `json:"pluginId"`
	Tags                  []int64 `json:"tags"`
}

type ImportListConfig struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Implementation   string  `json:"implementation"`
	Enable           bool    `json:"enable"`
	QualityProfileID int64   `json:"qualityProfileId"`
	RootFolderPath   string  `json:"rootFolderPath"`
	Monitor          string  `json:"monitor"`
	SearchOnAdd      bool    `json:"searchOnAdd"`
	URL              string  `json:"url"`
	Tags             []int64 `json:"tags"`
}

type ImportListOptions struct {
	ListSyncLevel string `json:"listSyncLevel"`
	ListSyncTag   int64  `json:"listSyncTag"`
}

type MetadataConfig struct {
	PreferredLanguage     string `json:"preferredLanguage"`
	CleanupMetadataImages bool   `json:"cleanupMetadataImages"`
}

type MetadataConsumer struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Implementation string `json:"implementation"`
	Enable         bool   `json:"enable"`
	SeriesImages   bool   `json:"seriesImages"`
	SeasonImages   bool   `json:"seasonImages"`
	EpisodeImages  bool   `json:"episodeImages"`
	MovieImages    bool   `json:"movieImages"`
	WriteNfo       bool   `json:"writeNfo"`
}

type RemotePathMapping struct {
	ID         int64  `json:"id"`
	Host       string `json:"host"`
	RemotePath string `json:"remotePath"`
	LocalPath  string `json:"localPath"`
}

type TagDetails struct {
	ID                int64   `json:"id"`
	Label             string  `json:"label"`
	SeriesIDs         []int64 `json:"seriesIds"`
	MovieIDs          []int64 `json:"movieIds"`
	IndexerIDs        []int64 `json:"indexerIds"`
	DownloadClientIDs []int64 `json:"downloadClientIds"`
	DelayProfileIDs   []int64 `json:"delayProfileIds"`
	ReleaseProfileIDs []int64 `json:"releaseProfileIds"`
	NotificationIDs   []int64 `json:"notificationIds"`
	ImportListIDs     []int64 `json:"importListIds"`
}

const standardEpisodeFormat = "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}"

// DefaultAppConfig returns the settings document used when nothing is stored.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Host: HostConfig{
			BindAddress:  "0.0.0.0",
			Port:         8989,
			URLBase:      "",
			InstanceName: "Replacearr",
			SSLPort:      9898,
			EnableSSL:    false,
		},
		Security: SecurityConfig{
			AuthenticationMethod:   "forms",
			AuthenticationRequired: "enabled",
			CertificateValidation:  "enabled",
		},
		Proxy: ProxyConfig{
			Type:                 "http",
			Port:                 8080,
			BypassLocalAddresses: true,
		},
		Logging: LoggingConfig{
			LogLevel:           "info",
			ConsoleLogLevel:    "info",
			LogSizeLimit:       1,
			HistoryCleanupDays: 30,
		},
		Analytics: AnalyticsConfig{Enabled: false},
		Updates: UpdateConfig{
			Branch:              "main",
			UpdateAutomatically: false,
			Mechanism:           "docker",
		},
		Backup: BackupConfig{
			Folder:    "Backups",
			Interval:  7,
			Retention: 28,
		},
		UI: UIConfig{
			FirstDayOfWeek:           0,
			CalendarWeekColumnHeader: "ddd M/D",
			ShortDateFormat:          "MMM D YYYY",
			LongDateFormat:           "dddd, MMMM D YYYY",
			TimeFormat:               "h:mm a",
			ShowRelativeDates:        true,
			Theme:                    "dark",
			UILanguage:               "en",
		},
		MediaManagement: MediaManagementConfig{
			ReplaceIllegalCharacters:      true,
			ColonReplacementFormat:        "smart",
			MultiEpisodeStyle:             "prefixedRange",
			StandardEpisodeFormat:         standardEpisodeFormat,
			DailyEpisodeFormat:            "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
			AnimeEpisodeFormat:            standardEpisodeFormat,
			SeriesFolderFormat:            "{Series Title}",
			SeasonFolderFormat:            "Season {season}",
			SpecialsFolderFormat:          "Specials",
			MovieFolderFormat:             "{Movie Title} ({Release Year})",
			MovieFileFormat:               "{Movie Title} ({Release Year}) {Quality Full}",
			EpisodeTitleRequired:          "always",
			MinimumFreeSpaceWhenImporting: 100,
			CopyUsingHardlinks:            true,
			ExtraFileExtensions:           "srt",
			DownloadPropersAndRepacks:     "preferAndUpgrade",
			AnalyzeVideoFiles:             true,
			RescanAfterRefresh:            "always",
			FileDate:                      "none",
			RecycleBinCleanupDays:         7,
			ChmodFolder:                   "755",
		},
		IndexerOptions: IndexerOptions{RssSyncInterval: 15},
		DownloadClientOptions: DownloadClientOptions{
			EnableCompletedDownloadHandling:           true,
			AutoRedownloadFailed:                      true,
			AutoRedownloadFailedFromInteractiveSearch: true,
			DownloadClientWorkingFolders:              "_UNPACK_|_FAILED_",
		},
		QualityDefinitions: BuiltinQualityDefinitions(),
		DelayProfiles:      []DelayProfile{},
		ReleaseProfiles:    []ReleaseProfile{},
		Notifications:      []NotificationConfig{},
		ImportLists:        []ImportListConfig{},
		ImportListOptions:  ImportListOptions{ListSyncLevel: "disabled"},
		Metadata: MetadataConfig{
			PreferredLanguage:     "english",
			CleanupMetadataImages: true,
		},
		MetadataConsumers:  []MetadataConsumer{},
		RemotePathMappings: []RemotePathMapping{},
	}
}

// BuiltinQualityDefinitions returns the built-in quality list. IDs match the
// quality items seeded onto default quality profiles.
func BuiltinQualityDefinitions() []QualityDefinition {
	return []QualityDefinition{
		qdef(0, "Unknown", "unknown", 0),
		qdef(1, "SDTV", "television", 480),
		qdef(2, "DVD", "dvd", 480),
		qdef(3, "WEBDL-480p", "web", 480),
		qdef(4, "HDTV-720p", "television", 720),
		qdef(5, "WEBDL-720p", "web", 720),
		qdef(6, "Bluray-720p", "bluray", 720),
		qdef(7, "WEBDL-1080p", "web", 1080),
		qdef(8, "HDTV-1080p", "television", 1080),
		qdef(9, "Bluray-1080p", "bluray", 1080),
		qdef(10, "Raw-HD", "television", 1080),
		qdef(11, "WEBRip-480p", "webRip", 480),
		qdef(12, "Bluray-480p", "bluray", 480),
		qdef(13, "WEBRip-720p", "webRip", 720),
		qdef(14, "WEBRip-1080p", "webRip", 1080),
		qdef(15, "Bluray-576p", "bluray", 576),
		qdef(16, "WEBRip-2160p", "webRip", 2160),
		qdef(17, "Bluray-1080p Remux", "bluray", 1080),
		qdef(18, "HDTV-2160p", "television", 2160),
		qdef(19, "WEBDL-2160p", "web", 2160),
		qdef(20, "Bluray-2160p", "bluray", 2160),
		qdef(21, "Bluray-2160p Remux", "bluray", 2160),
	}
}

func qdef(id int, name, source string, resolution int) QualityDefinition {
	return QualityDefinition{
		ID:         id,
		Name:       name,
		Title:      name,
		Source:     source,
		Resolution: resolution,
	}
}

// Normalize fills empty slices, pins authentication, and clamps ports and ids.
func (c *AppConfig) Normalize() {
	if c.QualityDefinitions == nil {
		c.QualityDefinitions = []QualityDefinition{}
	}
	if c.DelayProfiles == nil {
		c.DelayProfiles = []DelayProfile{}
	}
	for i := range c.DelayProfiles {
		c.DelayProfiles[i].Tags = nonNilInt64s(c.DelayProfiles[i].Tags)
	}
	if c.ReleaseProfiles == nil {
		c.ReleaseProfiles = []ReleaseProfile{}
	}
	for i := range c.ReleaseProfiles {
		c.ReleaseProfiles[i].Required = nonNilStrings(c.ReleaseProfiles[i].Required)
		c.ReleaseProfiles[i].Ignored = nonNilStrings(c.ReleaseProfiles[i].Ignored)
		c.ReleaseProfiles[i].Tags = nonNilInt64s(c.ReleaseProfiles[i].Tags)
	}
	if c.Notifications == nil {
		c.Notifications = []NotificationConfig{}
	}
	for i := range c.Notifications {
		c.Notifications[i].Tags = nonNilInt64s(c.Notifications[i].Tags)
	}
	if c.ImportLists == nil {
		c.ImportLists = []ImportListConfig{}
	}
	for i := range c.ImportLists {
		c.ImportLists[i].Tags = nonNilInt64s(c.ImportLists[i].Tags)
	}
	if c.MetadataConsumers == nil {
		c.MetadataConsumers = []MetadataConsumer{}
	}
	if c.RemotePathMappings == nil {
		c.RemotePathMappings = []RemotePathMapping{}
	}

	c.Security.AuthenticationMethod = "forms"
	if c.Updates.Mechanism == "" {
		c.Updates.Mechanism = "docker"
	}
	if c.IndexerOptions.RssSyncInterval < 0 {
		c.IndexerOptions.RssSyncInterval = 0
	}
	if c.IndexerOptions.RssSyncInterval > 120 {
		c.IndexerOptions.RssSyncInterval = 120
	}
	c.Host.Port = normalizePort(c.Host.Port, 8989)
	c.Host.SSLPort = normalizePort(c.Host.SSLPort, 9898)
	c.Proxy.Port = normalizePort(c.Proxy.Port, 8080)

	assignMissingIDs(len(c.DelayProfiles),
		func(i int) int64 { return c.DelayProfiles[i].ID },
		func(i int, id int64) { c.DelayProfiles[i].ID = id },
	)
	assignMissingIDs(len(c.ReleaseProfiles),
		func(i int) int64 { return c.ReleaseProfiles[i].ID },
		func(i int, id int64) { c.ReleaseProfiles[i].ID = id },
	)
	assignMissingIDs(len(c.Notifications),
		func(i int) int64 { return c.Notifications[i].ID },
		func(i int, id int64) { c.Notifications[i].ID = id },
	)
	assignMissingIDs(len(c.ImportLists),
		func(i int) int64 { return c.ImportLists[i].ID },
		func(i int, id int64) { c.ImportLists[i].ID = id },
	)
	assignMissingIDs(len(c.MetadataConsumers),
		func(i int) int64 { return c.MetadataConsumers[i].ID },
		func(i int, id int64) { c.MetadataConsumers[i].ID = id },
	)
	assignMissingIDs(len(c.RemotePathMappings),
		func(i int) int64 { return c.RemotePathMappings[i].ID },
		func(i int, id int64) { c.RemotePathMappings[i].ID = id },
	)
}

func nonNilInt64s(v []int64) []int64 {
	if v == nil {
		return []int64{}
	}
	return v
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func normalizePort(port, whenZero int) int {
	if port == 0 {
		port = whenZero
	}
	if port < 1 {
		return 1
	}
	if port > 65535 {
		return 65535
	}
	return port
}

func assignMissingIDs(n int, get func(i int) int64, set func(i int, id int64)) {
	var maxID int64
	for i := 0; i < n; i++ {
		if id := get(i); id > maxID {
			maxID = id
		}
	}
	next := maxID + 1
	if next < 1 {
		next = 1
	}
	for i := 0; i < n; i++ {
		if get(i) == 0 {
			set(i, next)
			next++
		}
	}
}
