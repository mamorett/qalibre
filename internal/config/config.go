// Package config manages the Qalibre configuration singleton stored in app.db's
// settings table.  It replaces cps/config_sql.py.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
)

// STABLE_VERSION mirrors cps/constants.py:STABLE_VERSION.
const STABLE_VERSION = "0.7.0-go"

// Default values (mirroring cps/config_sql.py defaults).
const (
	DefaultPort        = 8083
	DefaultBooksPerPage = 60
	DefaultTitleRegex  = `^(A|The|An|Der|Die|Das|Den|Ein|Eine|Einen|Dem|Des|Einem|Eines|Le|La|Les|L'|Un|Une)(\s+|(?<='))`
	DefaultLogLevel    = "INFO"
	DefaultLogFile     = "qalibre.log"
	DefaultAccessLog   = "access.log"

	LoginStandard = 0
	LoginLDAP     = 1
	LoginOAuth    = 2 // treated as standard (dropped)

	AdminUserRoles = RoleAdmin | RoleDownload | RoleUpload | RoleEdit | RolePasswd | RoleEditShelfs | RoleDeleteBooks | RoleViewer
	AdminUserSidebar = (SidebarList << 1) - 1
)

// Role bitfields (cps/constants.py:61-70).
const (
	RoleAdmin       = 1 << 0
	RoleDownload    = 1 << 1
	RoleUpload      = 1 << 2
	RoleEdit        = 1 << 3
	RolePasswd      = 1 << 4
	RoleAnonymous   = 1 << 5
	RoleEditShelfs  = 1 << 6
	RoleDeleteBooks = 1 << 7
	RoleViewer      = 1 << 8
)

// Sidebar bitfields (cps/constants.py:83-100).
const (
	DetailRandom       = 1 << 0
	SidebarLanguage    = 1 << 1
	SidebarSeries      = 1 << 2
	SidebarCategory    = 1 << 3
	SidebarHot         = 1 << 4
	SidebarRandom      = 1 << 5
	SidebarAuthor      = 1 << 6
	SidebarBestRated   = 1 << 7
	SidebarReadUnread  = 1 << 8
	SidebarRecent      = 1 << 9
	SidebarSorted      = 1 << 10
	MatureContent      = 1 << 11
	SidebarPublisher   = 1 << 12
	SidebarRating      = 1 << 13
	SidebarFormat      = 1 << 14
	SidebarArchived    = 1 << 15
	SidebarDownload    = 1 << 16
	SidebarList        = 1 << 17
)

// ExtensionsUpload lists allowed upload formats (audio removed).
var ExtensionsUpload = map[string]bool{
	"txt": true, "pdf": true, "epub": true, "kepub": true, "mobi": true,
	"azw": true, "azw3": true, "cbr": true, "cbz": true, "cbt": true,
	"cb7": true, "djvu": true, "djv": true, "prc": true, "doc": true,
	"docx": true, "fb2": true, "html": true, "rtf": true, "lit": true, "odt": true,
}

// ExtensionsConvertFrom and ExtensionsConvertTo mirror cps/constants.py.
var ExtensionsConvertFrom = []string{
	"pdf", "epub", "mobi", "azw3", "docx", "rtf", "fb2", "lit", "lrf",
	"txt", "htmlz", "odt", "cbz", "cbr", "prc",
}
var ExtensionsConvertTo = []string{
	"pdf", "epub", "mobi", "azw3", "docx", "rtf", "fb2", "lit", "lrf",
	"txt", "htmlz", "odt",
}

// Settings mirrors the settings table in app.db (cps/config_sql.py:57).
// Only kept (non-dropped) columns are exposed as typed fields.
// Dropped columns (mail_*, kobo_*, gdrive_*, oauth, email-register) are
// still present in the DB; we just don't touch them.
type Settings struct {
	// Core calibre settings
	ConfigCalibreDir       *string `db:"config_calibre_dir"`
	ConfigCalibreUUID      *string `db:"config_calibre_uuid"`
	ConfigCalibreSplit     bool    `db:"config_calibre_split"`
	ConfigCalibreSplitDir  *string `db:"config_calibre_split_dir"`
	ConfigPort             int     `db:"config_port"`
	ConfigExternalPort     int     `db:"config_external_port"`
	ConfigCertFile         *string `db:"config_certfile"`
	ConfigKeyFile          *string `db:"config_keyfile"`
	ConfigTrustedHosts     string  `db:"config_trustedhosts"`
	ConfigCalibreWebTitle  string  `db:"config_calibre_web_title"`
	ConfigBooksPerPage     int     `db:"config_books_per_page"`
	ConfigRandomBooks      int     `db:"config_random_books"`
	ConfigAuthorsMax       int     `db:"config_authors_max"`
	ConfigReadColumn       int     `db:"config_read_column"`
	ConfigTitleRegex       string  `db:"config_title_regex"`
	ConfigTheme            int     `db:"config_theme"`

	// Logging
	ConfigLogLevel      int    `db:"config_log_level"`
	ConfigLogFile       string `db:"config_logfile"`
	ConfigAccessLog     int    `db:"config_access_log"`
	ConfigAccessLogFile string `db:"config_access_logfile"`

	// Feature flags
	ConfigUploading  int  `db:"config_uploading"`
	ConfigAnonBrowse int  `db:"config_anonbrowse"`
	ConfigPublicReg  int  `db:"config_public_reg"`

	// New-user defaults
	ConfigDefaultRole         int    `db:"config_default_role"`
	ConfigDefaultShow         int    `db:"config_default_show"`
	ConfigDefaultLanguage     string `db:"config_default_language"`
	ConfigDefaultLocale       string `db:"config_default_locale"`
	ConfigColumnsToIgnore     string `db:"config_columns_to_ignore"`
	ConfigDeniedTags          string `db:"config_denied_tags"`
	ConfigAllowedTags         string `db:"config_allowed_tags"`
	ConfigRestrictedColumn    int    `db:"config_restricted_column"`
	ConfigDeniedColumnValue   string `db:"config_denied_column_value"`
	ConfigAllowedColumnValue  string `db:"config_allowed_column_value"`

	// Auth
	ConfigLoginType int `db:"config_login_type"`

	// Goodreads / Google Books
	ConfigUseGoodreads      bool   `db:"config_use_goodreads"`
	ConfigGoodreadsAPIKey   string `db:"config_goodreads_api_key"`
	ConfigGooglebooksAPIKey string `db:"config_googlebooks_api_key"`

	// LDAP
	ConfigLDAPProviderURL       string `db:"config_ldap_provider_url"`
	ConfigLDAPPort              int    `db:"config_ldap_port"`
	ConfigLDAPAuthentication    int    `db:"config_ldap_authentication"`
	ConfigLDAPServUsername      string `db:"config_ldap_serv_username"`
	ConfigLDAPServPasswordE     string `db:"config_ldap_serv_password_e"`
	ConfigLDAPServPassword      string `db:"config_ldap_serv_password"`
	ConfigLDAPEncryption        int    `db:"config_ldap_encryption"`
	ConfigLDAPCACertPath        string `db:"config_ldap_cacert_path"`
	ConfigLDAPCertPath          string `db:"config_ldap_cert_path"`
	ConfigLDAPKeyPath           string `db:"config_ldap_key_path"`
	ConfigLDAPDN                string `db:"config_ldap_dn"`
	ConfigLDAPUserObject        string `db:"config_ldap_user_object"`
	ConfigLDAPMemberUserObject  string `db:"config_ldap_member_user_object"`
	ConfigLDAPOpenLDAP          bool   `db:"config_ldap_openldap"`
	ConfigLDAPGroupObjectFilter string `db:"config_ldap_group_object_filter"`
	ConfigLDAPGroupMembersField string `db:"config_ldap_group_members_field"`
	ConfigLDAPGroupName         string `db:"config_ldap_group_name"`

	// Binaries / conversion
	ConfigKepubifyPath    *string `db:"config_kepubifypath"`
	ConfigConverterPath   *string `db:"config_converterpath"`
	ConfigBinariesDir     *string `db:"config_binariesdir"`
	ConfigCalibre         string  `db:"config_calibre"`
	ConfigRarfileLocation *string `db:"config_rarfile_location"`
	ConfigUploadFormats   string  `db:"config_upload_formats"`
	ConfigUnicodeFilename bool    `db:"config_unicode_filename"`
	ConfigEmbedMetadata   bool    `db:"config_embed_metadata"`

	// Reverse proxy login
	ConfigReverseProxyLoginHeaderName  string `db:"config_reverse_proxy_login_header_name"`
	ConfigAllowReverseProxyHeaderLogin bool   `db:"config_allow_reverse_proxy_header_login"`

	// Scheduler
	ScheduleStartTime             int  `db:"schedule_start_time"`
	ScheduleDuration              int  `db:"schedule_duration"`
	ScheduleGenerateBookCovers    bool `db:"schedule_generate_book_covers"`
	ScheduleGenerateSeriesCovers  bool `db:"schedule_generate_series_covers"`
	ScheduleReconnect             bool `db:"schedule_reconnect"`
	ScheduleMetadataBackup        bool `db:"schedule_metadata_backup"`

	// Security policy
	ConfigPasswordPolicy   bool   `db:"config_password_policy"`
	ConfigPasswordMinLength int   `db:"config_password_min_length"`
	ConfigPasswordNumber   bool   `db:"config_password_number"`
	ConfigPasswordLower    bool   `db:"config_password_lower"`
	ConfigPasswordUpper    bool   `db:"config_password_upper"`
	ConfigPasswordCharacter bool  `db:"config_password_character"`
	ConfigPasswordSpecial  bool   `db:"config_password_special"`
	ConfigSession          int    `db:"config_session"`
	ConfigRateLimiter      bool   `db:"config_ratelimiter"`
	ConfigLimiterURI       string `db:"config_limiter_uri"`
	ConfigLimiterOptions   string `db:"config_limiter_options"`
	ConfigCheckExtensions  bool   `db:"config_check_extensions"`
}

// Config wraps Settings with a db handle and dirty-tracking mutex.
type Config struct {
	mu       sync.RWMutex
	db       *sqlx.DB
	Settings Settings

	// Resolved binary paths (auto-detected at startup).
	EbookConvertBin string
	CalibreDBBin    string
	KepubifyBin     string
	UnrarBin        string
}

// Load reads (or seeds) the settings row from app.db.
func Load(db *sqlx.DB, calibreDir string) (*Config, error) {
	// Clean up any NULL values in existing settings rows to prevent scan errors
	sanitizeSettings(db)

	cfg := &Config{db: db}

	// Try to SELECT the row
	err := db.Unsafe().Get(&cfg.Settings, `SELECT * FROM settings LIMIT 1`)
	if err != nil {
		// Seed with defaults
		slog.Info("config: no settings row found, inserting defaults")
		if err2 := cfg.seedDefaults(); err2 != nil {
			return nil, fmt.Errorf("config: seed defaults: %w", err2)
		}
		// Clean up newly inserted defaults too
		sanitizeSettings(db)
		// Re-read
		if err3 := db.Unsafe().Get(&cfg.Settings, `SELECT * FROM settings LIMIT 1`); err3 != nil {
			return nil, fmt.Errorf("config: re-read after seed: %w", err3)
		}
	}

	// Auto-detect calibre dir if not set
	cfg.resolveCalibireDir(calibreDir)

	// Auto-detect binaries
	cfg.detectBinaries()

	return cfg, nil
}

// Get returns a read-locked copy of the settings.
func (c *Config) Get() Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Settings
}

// Save persists the current settings to the DB.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.NamedExec(`UPDATE settings SET
		config_calibre_dir=:config_calibre_dir,
		config_calibre_split=:config_calibre_split,
		config_calibre_split_dir=:config_calibre_split_dir,
		config_port=:config_port,
		config_certfile=:config_certfile,
		config_keyfile=:config_keyfile,
		config_trustedhosts=:config_trustedhosts,
		config_calibre_web_title=:config_calibre_web_title,
		config_books_per_page=:config_books_per_page,
		config_random_books=:config_random_books,
		config_authors_max=:config_authors_max,
		config_read_column=:config_read_column,
		config_title_regex=:config_title_regex,
		config_theme=:config_theme,
		config_log_level=:config_log_level,
		config_logfile=:config_logfile,
		config_access_log=:config_access_log,
		config_access_logfile=:config_access_logfile,
		config_uploading=:config_uploading,
		config_anonbrowse=:config_anonbrowse,
		config_public_reg=:config_public_reg,
		config_default_role=:config_default_role,
		config_default_show=:config_default_show,
		config_default_language=:config_default_language,
		config_default_locale=:config_default_locale,
		config_columns_to_ignore=:config_columns_to_ignore,
		config_denied_tags=:config_denied_tags,
		config_allowed_tags=:config_allowed_tags,
		config_restricted_column=:config_restricted_column,
		config_denied_column_value=:config_denied_column_value,
		config_allowed_column_value=:config_allowed_column_value,
		config_login_type=:config_login_type,
		config_use_goodreads=:config_use_goodreads,
		config_goodreads_api_key=:config_goodreads_api_key,
		config_googlebooks_api_key=:config_googlebooks_api_key,
		config_ldap_provider_url=:config_ldap_provider_url,
		config_ldap_port=:config_ldap_port,
		config_ldap_authentication=:config_ldap_authentication,
		config_ldap_serv_username=:config_ldap_serv_username,
		config_ldap_serv_password=:config_ldap_serv_password,
		config_ldap_encryption=:config_ldap_encryption,
		config_ldap_cacert_path=:config_ldap_cacert_path,
		config_ldap_cert_path=:config_ldap_cert_path,
		config_ldap_key_path=:config_ldap_key_path,
		config_ldap_dn=:config_ldap_dn,
		config_ldap_user_object=:config_ldap_user_object,
		config_ldap_member_user_object=:config_ldap_member_user_object,
		config_ldap_openldap=:config_ldap_openldap,
		config_ldap_group_object_filter=:config_ldap_group_object_filter,
		config_ldap_group_members_field=:config_ldap_group_members_field,
		config_ldap_group_name=:config_ldap_group_name,
		config_kepubifypath=:config_kepubifypath,
		config_converterpath=:config_converterpath,
		config_binariesdir=:config_binariesdir,
		config_calibre=:config_calibre,
		config_rarfile_location=:config_rarfile_location,
		config_upload_formats=:config_upload_formats,
		config_unicode_filename=:config_unicode_filename,
		config_embed_metadata=:config_embed_metadata,
		config_reverse_proxy_login_header_name=:config_reverse_proxy_login_header_name,
		config_allow_reverse_proxy_header_login=:config_allow_reverse_proxy_header_login,
		schedule_start_time=:schedule_start_time,
		schedule_duration=:schedule_duration,
		schedule_generate_book_covers=:schedule_generate_book_covers,
		schedule_generate_series_covers=:schedule_generate_series_covers,
		schedule_reconnect=:schedule_reconnect,
		schedule_metadata_backup=:schedule_metadata_backup,
		config_password_policy=:config_password_policy,
		config_password_min_length=:config_password_min_length,
		config_password_number=:config_password_number,
		config_password_lower=:config_password_lower,
		config_password_upper=:config_password_upper,
		config_password_character=:config_password_character,
		config_password_special=:config_password_special,
		config_session=:config_session,
		config_ratelimiter=:config_ratelimiter,
		config_limiter_uri=:config_limiter_uri,
		config_limiter_options=:config_limiter_options,
		config_check_extensions=:config_check_extensions
	WHERE id=1`, c.Settings)
	return err
}

// Update applies fn to the settings under a write lock and saves.
func (c *Config) Update(fn func(*Settings)) error {
	c.mu.Lock()
	fn(&c.Settings)
	c.mu.Unlock()
	return c.Save()
}

// GetCalibreDir returns the resolved calibre directory.
func (c *Config) GetCalibreDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Settings.ConfigCalibreDir != nil && *c.Settings.ConfigCalibreDir != "" {
		return *c.Settings.ConfigCalibreDir
	}
	return ""
}

// HasFlag checks a bitfield.
func HasFlag(value, bit int) bool { return bit != 0 && (value&bit) == bit }

func (c *Config) resolveCalibireDir(cliCalibireDir string) {
	// Priority: CLI arg > DB setting > /library > embedded library
	if cliCalibireDir != "" {
		c.mu.Lock()
		c.Settings.ConfigCalibreDir = &cliCalibireDir
		c.mu.Unlock()
		return
	}
	if c.Settings.ConfigCalibreDir != nil && *c.Settings.ConfigCalibreDir != "" {
		return
	}
	// Auto-detect: /library
	if _, err := os.Stat("/library/metadata.db"); err == nil {
		dir := "/library"
		c.mu.Lock()
		c.Settings.ConfigCalibreDir = &dir
		c.mu.Unlock()
		slog.Info("config: auto-detected calibre dir", "dir", dir)
	}
}

func (c *Config) detectBinaries() {
	s := c.Settings
	var binDir string
	if s.ConfigBinariesDir != nil {
		binDir = *s.ConfigBinariesDir
	}

	// ebook-convert
	if s.ConfigConverterPath != nil && *s.ConfigConverterPath != "" {
		c.EbookConvertBin = *s.ConfigConverterPath
	} else {
		c.EbookConvertBin = lookBinary("ebook-convert", binDir)
	}
	if c.EbookConvertBin != "" {
		slog.Info("config: ebook-convert", "path", c.EbookConvertBin)
	}

	// calibredb
	c.CalibreDBBin = lookBinary("calibredb", binDir)
	if c.CalibreDBBin != "" {
		slog.Info("config: calibredb", "path", c.CalibreDBBin)
	}

	// kepubify
	if s.ConfigKepubifyPath != nil && *s.ConfigKepubifyPath != "" {
		c.KepubifyBin = *s.ConfigKepubifyPath
	} else {
		c.KepubifyBin = lookBinary("kepubify", binDir)
	}

	// unrar
	if s.ConfigRarfileLocation != nil && *s.ConfigRarfileLocation != "" {
		c.UnrarBin = *s.ConfigRarfileLocation
	} else {
		c.UnrarBin = lookBinary("unrar", binDir)
		if c.UnrarBin == "" {
			c.UnrarBin = lookBinary("unar", binDir)
		}
	}
}

func lookBinary(name, binDir string) string {
	if binDir != "" {
		candidate := filepath.Join(binDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

func (c *Config) seedDefaults() error {
	uploadFormats := strings.Join(func() []string {
		out := make([]string, 0, len(ExtensionsUpload))
		for k := range ExtensionsUpload {
			out = append(out, k)
		}
		return out
	}(), ",")

	_, err := c.db.Exec(`INSERT OR IGNORE INTO settings (
		id, config_calibre_web_title, config_books_per_page, config_random_books,
		config_authors_max, config_read_column, config_title_regex, config_theme,
		config_log_level, config_logfile, config_access_log, config_access_logfile,
		config_uploading, config_anonbrowse, config_public_reg,
		config_default_role, config_default_show, config_default_language, config_default_locale,
		config_denied_tags, config_allowed_tags, config_restricted_column,
		config_denied_column_value, config_allowed_column_value,
		config_login_type, config_ldap_port, config_ldap_authentication,
		config_ldap_provider_url, config_ldap_serv_username, config_ldap_dn,
		config_ldap_user_object, config_ldap_member_user_object,
		config_ldap_group_object_filter, config_ldap_group_members_field, config_ldap_group_name,
		config_upload_formats, config_unicode_filename, config_embed_metadata,
		schedule_start_time, schedule_duration, config_password_policy,
		config_password_min_length, config_password_number, config_password_lower,
		config_password_upper, config_password_character, config_password_special,
		config_session, config_ratelimiter, config_limiter_uri, config_limiter_options,
		config_check_extensions, config_trustedhosts, config_port, config_external_port,
		config_goodreads_api_key, config_googlebooks_api_key, config_calibre,
		config_ldap_cacert_path, config_ldap_cert_path, config_ldap_key_path,
		config_ldap_openldap, config_reverse_proxy_login_header_name,
		config_allow_reverse_proxy_header_login, config_columns_to_ignore
	) VALUES (
		1, 'Qalibre', 60, 4,
		0, 0, ?, 0,
		20, 'qalibre.log', 0, 'access.log',
		0, 0, 0,
		0, ?, 'all', 'en',
		'', '', 0,
		'', '',
		0, 389, 0,
		'example.org', 'cn=admin,dc=example,dc=org', 'dc=example,dc=org',
		'uid=%s', '',
		'(&(objectclass=posixGroup)(cn=%s))', 'memberUid', 'calibreweb',
		?, 0, 1,
		4, 10, 1,
		8, 1, 1,
		1, 1, 1,
		1, 1, '', '',
		1, '', ?, ?,
		'', '', '',
		'', '', '',
		1, '',
		0, ''
	)`,
		DefaultTitleRegex,
		AdminUserSidebar,
		uploadFormats,
		DefaultPort,
		DefaultPort,
	)
	return err
}

func sanitizeSettings(db *sqlx.DB) {
	_, _ = db.Exec(`UPDATE settings SET
		config_calibre_split = COALESCE(config_calibre_split, 0),
		config_port = COALESCE(config_port, 8083),
		config_external_port = COALESCE(config_external_port, 8083),
		config_trustedhosts = COALESCE(config_trustedhosts, ''),
		config_calibre_web_title = COALESCE(config_calibre_web_title, 'Qalibre'),
		config_books_per_page = COALESCE(config_books_per_page, 60),
		config_random_books = COALESCE(config_random_books, 4),
		config_authors_max = COALESCE(config_authors_max, 0),
		config_read_column = COALESCE(config_read_column, 0),
		config_title_regex = COALESCE(config_title_regex, ''),
		config_theme = COALESCE(config_theme, 0),
		config_log_level = COALESCE(config_log_level, 20),
		config_logfile = COALESCE(config_logfile, 'qalibre.log'),
		config_access_log = COALESCE(config_access_log, 0),
		config_access_logfile = COALESCE(config_access_logfile, 'access.log'),
		config_uploading = COALESCE(config_uploading, 0),
		config_anonbrowse = COALESCE(config_anonbrowse, 0),
		config_public_reg = COALESCE(config_public_reg, 0),
		config_default_role = COALESCE(config_default_role, 0),
		config_default_show = COALESCE(config_default_show, 0),
		config_default_language = COALESCE(config_default_language, 'all'),
		config_default_locale = COALESCE(config_default_locale, 'en'),
		config_columns_to_ignore = COALESCE(config_columns_to_ignore, ''),
		config_denied_tags = COALESCE(config_denied_tags, ''),
		config_allowed_tags = COALESCE(config_allowed_tags, ''),
		config_restricted_column = COALESCE(config_restricted_column, 0),
		config_denied_column_value = COALESCE(config_denied_column_value, ''),
		config_allowed_column_value = COALESCE(config_allowed_column_value, ''),
		config_login_type = COALESCE(config_login_type, 0),
		config_use_goodreads = COALESCE(config_use_goodreads, 0),
		config_goodreads_api_key = COALESCE(config_goodreads_api_key, ''),
		config_googlebooks_api_key = COALESCE(config_googlebooks_api_key, ''),
		config_ldap_provider_url = COALESCE(config_ldap_provider_url, ''),
		config_ldap_port = COALESCE(config_ldap_port, 389),
		config_ldap_authentication = COALESCE(config_ldap_authentication, 0),
		config_ldap_serv_username = COALESCE(config_ldap_serv_username, ''),
		config_ldap_serv_password_e = COALESCE(config_ldap_serv_password_e, ''),
		config_ldap_serv_password = COALESCE(config_ldap_serv_password, ''),
		config_ldap_encryption = COALESCE(config_ldap_encryption, 0),
		config_ldap_cacert_path = COALESCE(config_ldap_cacert_path, ''),
		config_ldap_cert_path = COALESCE(config_ldap_cert_path, ''),
		config_ldap_key_path = COALESCE(config_ldap_key_path, ''),
		config_ldap_dn = COALESCE(config_ldap_dn, ''),
		config_ldap_user_object = COALESCE(config_ldap_user_object, ''),
		config_ldap_member_user_object = COALESCE(config_ldap_member_user_object, ''),
		config_ldap_openldap = COALESCE(config_ldap_openldap, 0),
		config_ldap_group_object_filter = COALESCE(config_ldap_group_object_filter, ''),
		config_ldap_group_members_field = COALESCE(config_ldap_group_members_field, ''),
		config_ldap_group_name = COALESCE(config_ldap_group_name, ''),
		config_calibre = COALESCE(config_calibre, ''),
		config_upload_formats = COALESCE(config_upload_formats, ''),
		config_unicode_filename = COALESCE(config_unicode_filename, 0),
		config_embed_metadata = COALESCE(config_embed_metadata, 0),
		config_reverse_proxy_login_header_name = COALESCE(config_reverse_proxy_login_header_name, ''),
		config_allow_reverse_proxy_header_login = COALESCE(config_allow_reverse_proxy_header_login, 0),
		schedule_start_time = COALESCE(schedule_start_time, 0),
		schedule_duration = COALESCE(schedule_duration, 0),
		schedule_generate_book_covers = COALESCE(schedule_generate_book_covers, 0),
		schedule_generate_series_covers = COALESCE(schedule_generate_series_covers, 0),
		schedule_reconnect = COALESCE(schedule_reconnect, 0),
		schedule_metadata_backup = COALESCE(schedule_metadata_backup, 0),
		config_password_policy = COALESCE(config_password_policy, 0),
		config_password_min_length = COALESCE(config_password_min_length, 8),
		config_password_number = COALESCE(config_password_number, 0),
		config_password_lower = COALESCE(config_password_lower, 0),
		config_password_upper = COALESCE(config_password_upper, 0),
		config_password_character = COALESCE(config_password_character, 0),
		config_password_special = COALESCE(config_password_special, 0),
		config_session = COALESCE(config_session, 0),
		config_ratelimiter = COALESCE(config_ratelimiter, 0),
		config_limiter_uri = COALESCE(config_limiter_uri, ''),
		config_limiter_options = COALESCE(config_limiter_options, ''),
		config_check_extensions = COALESCE(config_check_extensions, 0)`)
}

