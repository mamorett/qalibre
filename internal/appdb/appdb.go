package appdb

import (
	"database/sql"
	"encoding/json"
	"time"
)

// User table schema
type User struct {
	ID                  int    `db:"id"`
	Name                string `db:"name"`
	Email               string `db:"email"`
	Role                int    `db:"role"`
	Password            string `db:"password"`
	KindleMail          string `db:"kindle_mail"`
	Locale              string `db:"locale"`
	SidebarView         int    `db:"sidebar_view"`
	DefaultLanguage     string `db:"default_language"`
	DeniedTags          string `db:"denied_tags"`
	AllowedTags         string `db:"allowed_tags"`
	DeniedColumnValue   string `db:"denied_column_value"`
	AllowedColumnValue  string `db:"allowed_column_value"`
	ViewSettings        string `db:"view_settings"` // Stored as JSON string
	KoboOnlyShelvesSync int    `db:"kobo_only_shelves_sync"`
}

// GetViewSettings unmarshals the ViewSettings field
func (u *User) GetViewSettings() (map[string]interface{}, error) {
	var m map[string]interface{}
	if u.ViewSettings == "" || u.ViewSettings == "{}" {
		return make(map[string]interface{}), nil
	}
	err := json.Unmarshal([]byte(u.ViewSettings), &m)
	return m, err
}

// SetViewSettings marshals view settings to the ViewSettings field
func (u *User) SetViewSettings(m map[string]interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	u.ViewSettings = string(b)
	return nil
}

// Shelf table schema
type Shelf struct {
	ID           int       `db:"id"`
	UUID         string    `db:"uuid"`
	Name         string    `db:"name"`
	IsPublic     int       `db:"is_public"`
	UserID       int       `db:"user_id"`
	KoboSync     bool      `db:"kobo_sync"`
	Created      time.Time `db:"created"`
	LastModified time.Time `db:"last_modified"`
}

// BookShelfLink table schema (book_shelf_link)
type BookShelfLink struct {
	ID        int       `db:"id"`
	BookID    int       `db:"book_id"`
	Order     int       `db:"order"`
	Shelf     int       `db:"shelf"` // Shelf ID
	DateAdded time.Time `db:"date_added"`
}

// ReadBook table schema (book_read_link)
type ReadBook struct {
	ID                     int          `db:"id"`
	BookID                 int          `db:"book_id"`
	UserID                 int          `db:"user_id"`
	ReadStatus             int          `db:"read_status"`
	LastModified           time.Time    `db:"last_modified"`
	LastTimeStartedReading sql.NullTime `db:"last_time_started_reading"`
	TimesStartedReading    int          `db:"times_started_reading"`
}

const (
	ReadStatusUnread     = 0
	ReadStatusFinished   = 1
	ReadStatusInProgress = 2
)

// Bookmark table schema
type Bookmark struct {
	ID          int    `db:"id"`
	UserID      int    `db:"user_id"`
	BookID      int    `db:"book_id"`
	Format      string `db:"format"`
	BookmarkKey string `db:"bookmark_key"`
}

// ArchivedBook table schema
type ArchivedBook struct {
	ID           int       `db:"id"`
	UserID       int       `db:"user_id"`
	BookID       int       `db:"book_id"`
	IsArchived   bool      `db:"is_archived"`
	LastModified time.Time `db:"last_modified"`
}

// Downloads table schema
type Downloads struct {
	ID     int `db:"id"`
	BookID int `db:"book_id"`
	UserID int `db:"user_id"`
}

// Registration table schema
type Registration struct {
	ID     int    `db:"id"`
	Domain string `db:"domain"`
	Allow  int    `db:"allow"`
}

// Thumbnail table schema
type Thumbnail struct {
	ID          int          `db:"id"`
	EntityID    int          `db:"entity_id"`
	UUID        string       `db:"uuid"`
	Format      string       `db:"format"`
	Type        int16        `db:"type"`
	Resolution  int16        `db:"resolution"`
	Filename    string       `db:"filename"`
	GeneratedAt time.Time    `db:"generated_at"`
	Expiration  sql.NullTime `db:"expiration"`
}

// UserSession table schema (user_session)
type UserSession struct {
	ID         int    `db:"id"`
	UserID     int    `db:"user_id"`
	SessionKey string `db:"session_key"`
	Random     string `db:"random"`
	Expiry     int64  `db:"expiry"`
}

// FlaskSettings table schema (flask_settings)
type FlaskSettings struct {
	ID              int    `db:"id"`
	FlaskSessionKey []byte `db:"flask_session_key"`
}

type Dataset struct {
	ID              int           `db:"id"               json:"id"`
	UUID            string        `db:"uuid"             json:"uuid"`
	Name            string        `db:"name"             json:"name"`
	Description     string        `db:"description"      json:"description"`
	ExportDirectory string        `db:"export_directory" json:"export_directory"`
	UserID          sql.NullInt64 `db:"user_id"          json:"user_id,omitempty"`
	S3Endpoint      string        `db:"s3_endpoint"      json:"s3_endpoint"`
	S3Region        string        `db:"s3_region"        json:"s3_region"`
	S3Bucket        string        `db:"s3_bucket"        json:"s3_bucket"`
	S3AccessKey     string        `db:"s3_access_key"    json:"s3_access_key"`
	S3SecretKey     string        `db:"s3_secret_key"    json:"s3_secret_key"`
	S3UseSSL        bool          `db:"s3_use_ssl"       json:"s3_use_ssl"`
	S3ForcePathStyle bool         `db:"s3_force_path_style" json:"s3_force_path_style"`
	Created         time.Time     `db:"created"          json:"created"`
	LastModified    time.Time     `db:"last_modified"    json:"last_modified"`
}

type DatasetBook struct {
	ID        int       `db:"id"         json:"id"`
	DatasetID int       `db:"dataset_id" json:"dataset_id"`
	BookID    int       `db:"book_id"    json:"book_id"`
	SortOrder int       `db:"sort_order" json:"sort_order"`
	AddedAt   time.Time `db:"added_at"   json:"added_at"`
}

type DatasetMetadata struct {
	ID        int    `db:"id"         json:"id"`
	DatasetID int    `db:"dataset_id" json:"dataset_id"`
	Key       string `db:"key"        json:"key"`
	Value     string `db:"value"      json:"value"`      // raw text
	ValueType string `db:"value_type" json:"value_type"` // string|number|timestamp|path
	SortOrder int    `db:"sort_order" json:"sort_order"`
}
