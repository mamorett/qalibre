package calibredb

import (
	"time"
)

// LibraryId table
type LibraryId struct {
	ID   int    `db:"id"`
	UUID string `db:"uuid"`
}

// Identifier table (identifiers)
type Identifier struct {
	ID     int    `db:"id"`
	Type   string `db:"type"`
	Val    string `db:"val"`
	BookID int    `db:"book"`
}

// Comment table (comments)
type Comment struct {
	ID     int    `db:"id"`
	BookID int    `db:"book"`
	Text   string `db:"text"`
}

// Tag table (tags)
type Tag struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

// Author table (authors)
type Author struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Sort string `db:"sort"`
	Link string `db:"link"`
}

// Series table
type Series struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Sort string `db:"sort"`
}

// Rating table (ratings)
type Rating struct {
	ID     int `db:"id"`
	Rating int `db:"rating"`
}

// Language table (languages)
type Language struct {
	ID       int    `db:"id"`
	LangCode string `db:"lang_code"`
}

// Publisher table (publishers)
type Publisher struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Sort string `db:"sort"`
}

// Data table (data)
type Data struct {
	ID               int    `db:"id"`
	BookID           int    `db:"book"`
	Format           string `db:"format"`
	UncompressedSize int64  `db:"uncompressed_size"`
	Name             string `db:"name"`
}

// MetadataDirtied table (metadata_dirtied)
type MetadataDirtied struct {
	ID     int `db:"id"`
	BookID int `db:"book"`
}

// Book table (books)
type Book struct {
	ID           int       `db:"id"`
	Title        string    `db:"title"`
	Sort         string    `db:"sort"`
	AuthorSort   string    `db:"author_sort"`
	Timestamp    time.Time `db:"timestamp"`
	Pubdate      time.Time `db:"pubdate"`
	SeriesIndex  string    `db:"series_index"`
	LastModified time.Time `db:"last_modified"`
	Path         string    `db:"path"`
	HasCover     int       `db:"has_cover"`
	UUID         string    `db:"uuid"`
}

// CustomColumnDef holds the schema information for a Calibre custom column.
type CustomColumnDef struct {
	ID         int    `db:"id"`
	Label      string `db:"label"`
	Name       string `db:"name"`
	DataType   string `db:"datatype"`
	IsMultiple bool   `db:"is_multiple"`
	Normalized bool   `db:"normalized"`
	Display    string `db:"display"` // JSON config
}
