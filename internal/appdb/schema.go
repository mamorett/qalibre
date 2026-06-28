package appdb

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/config"
)

// InitSchema creates all tables in app.db if they do not exist and seeds the default admin and guest users.
func InitSchema(db *sqlx.DB, hashedAdminPassword string) error {
	schemaQueries := []string{
		`CREATE TABLE IF NOT EXISTS user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			email TEXT UNIQUE DEFAULT "",
			role INTEGER DEFAULT 0,
			password TEXT,
			kindle_mail TEXT DEFAULT "",
			locale TEXT DEFAULT "en",
			sidebar_view INTEGER DEFAULT 1,
			default_language TEXT DEFAULT "all",
			denied_tags TEXT DEFAULT "",
			allowed_tags TEXT DEFAULT "",
			denied_column_value TEXT DEFAULT "",
			allowed_column_value TEXT DEFAULT "",
			view_settings TEXT DEFAULT "{}",
			kobo_only_shelves_sync INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS shelf (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT,
			name TEXT,
			is_public INTEGER DEFAULT 0,
			user_id INTEGER,
			kobo_sync INTEGER DEFAULT 0,
			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_modified TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS book_shelf_link (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER,
			[order] INTEGER,
			shelf INTEGER,
			date_added TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS book_read_link (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER,
			user_id INTEGER,
			read_status INTEGER DEFAULT 0 NOT NULL,
			last_modified TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_time_started_reading TIMESTAMP,
			times_started_reading INTEGER DEFAULT 0 NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS bookmark (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			book_id INTEGER,
			format TEXT,
			bookmark_key TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS archived_book (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			book_id INTEGER,
			is_archived INTEGER DEFAULT 0,
			last_modified TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER,
			user_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS registration (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT,
			allow INTEGER DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS thumbnail (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_id INTEGER,
			uuid TEXT UNIQUE,
			format TEXT DEFAULT 'jpeg',
			type INTEGER DEFAULT 1,
			resolution INTEGER DEFAULT 1,
			filename TEXT,
			generated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expiration TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_session (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			session_key TEXT DEFAULT "",
			random TEXT DEFAULT "",
			expiry INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS flask_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			flask_session_key BLOB
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mail_server TEXT,
			mail_port INTEGER,
			mail_use_ssl INTEGER,
			mail_login TEXT,
			mail_password_e TEXT,
			mail_password TEXT,
			mail_from TEXT,
			mail_size INTEGER,
			mail_server_type INTEGER,
			mail_gmail_token TEXT,
			config_calibre_dir TEXT,
			config_calibre_uuid TEXT,
			config_calibre_split INTEGER DEFAULT 0,
			config_calibre_split_dir TEXT,
			config_port INTEGER,
			config_external_port INTEGER,
			config_certfile TEXT,
			config_keyfile TEXT,
			config_trustedhosts TEXT,
			config_calibre_web_title TEXT,
			config_books_per_page INTEGER,
			config_random_books INTEGER,
			config_authors_max INTEGER,
			config_read_column INTEGER,
			config_title_regex TEXT,
			config_theme INTEGER,
			config_log_level INTEGER,
			config_logfile TEXT,
			config_access_log INTEGER,
			config_access_logfile TEXT,
			config_uploading INTEGER,
			config_anonbrowse INTEGER,
			config_public_reg INTEGER,
			config_remote_login INTEGER,
			config_kobo_sync INTEGER,
			config_default_role INTEGER,
			config_default_show INTEGER,
			config_default_language TEXT,
			config_default_locale TEXT,
			config_columns_to_ignore TEXT,
			config_denied_tags TEXT,
			config_allowed_tags TEXT,
			config_restricted_column INTEGER,
			config_denied_column_value TEXT,
			config_allowed_column_value TEXT,
			config_use_google_drive INTEGER,
			config_google_drive_folder TEXT,
			config_google_drive_watch_changes_response TEXT,
			config_use_goodreads INTEGER,
			config_goodreads_api_key TEXT,
			config_googlebooks_api_key TEXT,
			config_register_email INTEGER,
			config_login_type INTEGER,
			config_kobo_proxy INTEGER,
			config_ldap_provider_url TEXT,
			config_ldap_port INTEGER,
			config_ldap_authentication INTEGER,
			config_ldap_serv_username TEXT,
			config_ldap_serv_password_e TEXT,
			config_ldap_serv_password TEXT,
			config_ldap_encryption INTEGER,
			config_ldap_cacert_path TEXT,
			config_ldap_cert_path TEXT,
			config_ldap_key_path TEXT,
			config_ldap_dn TEXT,
			config_ldap_user_object TEXT,
			config_ldap_member_user_object TEXT,
			config_ldap_openldap INTEGER,
			config_ldap_group_object_filter TEXT,
			config_ldap_group_members_field TEXT,
			config_ldap_group_name TEXT,
			config_kepubifypath TEXT,
			config_converterpath TEXT,
			config_binariesdir TEXT,
			config_calibre TEXT,
			config_rarfile_location TEXT,
			config_upload_formats TEXT,
			config_unicode_filename INTEGER,
			config_embed_metadata INTEGER,
			config_updatechannel INTEGER,
			config_reverse_proxy_login_header_name TEXT,
			config_allow_reverse_proxy_header_login INTEGER,
			schedule_start_time INTEGER,
			schedule_duration INTEGER,
			schedule_generate_book_covers INTEGER,
			schedule_generate_series_covers INTEGER,
			schedule_reconnect INTEGER,
			schedule_metadata_backup INTEGER,
			config_password_policy INTEGER,
			config_password_min_length INTEGER,
			config_password_number INTEGER,
			config_password_lower INTEGER,
			config_password_upper INTEGER,
			config_password_character INTEGER,
			config_password_special INTEGER,
			config_session INTEGER,
			config_ratelimiter INTEGER,
			config_limiter_uri TEXT,
			config_limiter_options TEXT,
			config_check_extensions INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS dataset (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid             TEXT UNIQUE,
			name             TEXT NOT NULL,
			description      TEXT DEFAULT '',
			export_directory TEXT NOT NULL DEFAULT '',
			user_id          INTEGER,
			created          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_modified    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS dataset_book (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			dataset_id INTEGER NOT NULL,
			book_id    INTEGER NOT NULL,
			sort_order INTEGER DEFAULT 0,
			added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (dataset_id, book_id)
		)`,
		`CREATE TABLE IF NOT EXISTS dataset_metadata (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			dataset_id INTEGER NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT,
			value_type TEXT NOT NULL DEFAULT 'string',
			sort_order INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dataset_book_dataset ON dataset_book(dataset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dataset_metadata_dataset ON dataset_metadata(dataset_id)`,
	}

	for _, query := range schemaQueries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	// Idempotent migration check for export_directory in dataset table
	var hasExportDir bool
	rows, err := db.Query("PRAGMA table_info(dataset)")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name string
			var typeStr string
			var notnull int
			var dfltVal interface{}
			var pk int
			if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltVal, &pk); err == nil {
				if name == "export_directory" {
					hasExportDir = true
				}
			}
		}
	}
	if !hasExportDir {
		if _, err := db.Exec("ALTER TABLE dataset ADD COLUMN export_directory TEXT NOT NULL DEFAULT ''"); err != nil {
			slog.Error("appdb: failed to migrate dataset table to add export_directory", "err", err)
		} else {
			slog.Info("appdb: added export_directory column to dataset table")
		}
	}

	// Seed domain registration if empty
	var regCount int
	if err := db.Get(&regCount, "SELECT COUNT(*) FROM registration"); err == nil && regCount == 0 {
		_, _ = db.Exec("INSERT INTO registration (domain, allow) VALUES ('%.%', 1)")
	}

	// Seed admin user if not exists
	var adminCount int
	if err := db.Get(&adminCount, "SELECT COUNT(*) FROM user WHERE name = 'admin'"); err == nil && adminCount == 0 {
		_, err = db.Exec(`INSERT INTO user (
			name, email, role, password, locale, sidebar_view, default_language,
			denied_tags, allowed_tags, denied_column_value, allowed_column_value
		) VALUES (
			'admin', 'admin@example.com', ?, ?, 'en', ?, 'all', '', '', '', ''
		)`,
			config.RoleAdmin|config.RoleDownload|config.RoleUpload|config.RoleEdit|config.RolePasswd|config.RoleEditShelfs|config.RoleDeleteBooks|config.RoleViewer,
			hashedAdminPassword,
			(1<<18)-1, // AdminUserSidebar
		)
		if err != nil {
			slog.Error("appdb: failed to seed admin user", "err", err)
		} else {
			slog.Info("appdb: seeded admin user (admin/admin123)")
		}
	}

	// Seed Guest user if not exists
	var guestCount int
	if err := db.Get(&guestCount, "SELECT COUNT(*) FROM user WHERE name = 'Guest'"); err == nil && guestCount == 0 {
		_, err = db.Exec(`INSERT INTO user (
			name, email, role, password, locale, sidebar_view, default_language,
			denied_tags, allowed_tags, denied_column_value, allowed_column_value
		) VALUES (
			'Guest', 'guest@example.com', ?, '', 'en', 1, 'all', '', '', '', ''
		)`, config.RoleAnonymous)
		if err != nil {
			slog.Error("appdb: failed to seed guest user", "err", err)
		} else {
			slog.Info("appdb: seeded Guest user")
		}
	}

	return nil
}
