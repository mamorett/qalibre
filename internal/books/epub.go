package books

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
)

// ContainerXml structure of META-INF/container.xml
type ContainerXml struct {
	XMLName   xml.Name   `xml:"container"`
	Rootfiles []Rootfile `xml:"rootfiles>rootfile"`
}

type Rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

// OpfXml structure of content.opf
type OpfXml struct {
	XMLName  xml.Name     `xml:"package"`
	Metadata OpfMetadata  `xml:"metadata"`
	Manifest OpfManifest  `xml:"manifest"`
}

type OpfMetadata struct {
	Title       string        `xml:"title"`
	Creator     []OpfCreator  `xml:"creator"`
	Language    []string      `xml:"language"`
	Description string        `xml:"description"`
	Publisher   string        `xml:"publisher"`
	Meta        []OpfMetaTag  `xml:"meta"`
	Identifier  []OpfIdentTag `xml:"identifier"`
}

type OpfCreator struct {
	Name string `xml:",chardata"`
	Role string `xml:"role,attr"`
}

type OpfMetaTag struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type OpfIdentTag struct {
	Scheme string `xml:"scheme,attr"`
	Id     string `xml:"id,attr"`
	Value  string `xml:",chardata"`
}

type OpfManifest struct {
	Items []OpfItem `xml:"item"`
}

type OpfItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// BookMeta struct to hold parsed metadata.
type BookMeta struct {
	Title       string
	Authors     []string
	Language    string
	Description string
	Publisher   string
	Series      string
	SeriesIndex string
	ISBN        string
	CoverData   []byte
	CoverExt    string
}

// ExtractEpubMetadata reads metadata from a zip.Reader (EPUB).
func ExtractEpubMetadata(r *zip.Reader) (*BookMeta, error) {
	meta := &BookMeta{
		Title:   "Unknown",
		Authors: []string{"Unknown"},
	}

	// 1. Read META-INF/container.xml to find OPF path
	var opfPath string
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			var container ContainerXml
			err = xml.NewDecoder(rc).Decode(&container)
			rc.Close()
			if err == nil && len(container.Rootfiles) > 0 {
				opfPath = container.Rootfiles[0].FullPath
			}
			break
		}
	}

	if opfPath == "" {
		// Fallback/guess if container.xml is missing
		for _, f := range r.File {
			if strings.HasSuffix(f.Name, ".opf") {
				opfPath = f.Name
				break
			}
		}
	}

	if opfPath == "" {
		return meta, nil // Couldn't find OPF
	}

	// 2. Read OPF file
	var opf OpfXml
	var opfFile *zip.File
	for _, f := range r.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}

	if opfFile == nil {
		return meta, nil
	}

	rc, err := opfFile.Open()
	if err != nil {
		return nil, err
	}
	err = xml.NewDecoder(rc).Decode(&opf)
	rc.Close()
	if err != nil {
		return nil, err
	}

	// Map metadata
	if opf.Metadata.Title != "" {
		meta.Title = strings.TrimSpace(opf.Metadata.Title)
	}

	if len(opf.Metadata.Creator) > 0 {
		var authors []string
		for _, c := range opf.Metadata.Creator {
			if name := strings.TrimSpace(c.Name); name != "" {
				authors = append(authors, name)
			}
		}
		if len(authors) > 0 {
			meta.Authors = authors
		}
	}

	if len(opf.Metadata.Language) > 0 {
		meta.Language = opf.Metadata.Language[0]
	}

	meta.Description = strings.TrimSpace(opf.Metadata.Description)
	meta.Publisher = strings.TrimSpace(opf.Metadata.Publisher)

	// Series & series_index
	for _, m := range opf.Metadata.Meta {
		if m.Name == "calibre:series" {
			meta.Series = m.Content
		}
		if m.Name == "calibre:series_index" {
			meta.SeriesIndex = m.Content
		}
	}

	// Identifiers (ISBN etc)
	for _, id := range opf.Metadata.Identifier {
		if strings.ToLower(id.Scheme) == "isbn" || strings.Contains(strings.ToLower(id.Id), "isbn") {
			meta.ISBN = strings.TrimSpace(id.Value)
		}
	}

	// 3. Find Cover Image Href
	var coverID string
	for _, m := range opf.Metadata.Meta {
		if m.Name == "cover" {
			coverID = m.Content
		}
	}

	var coverHref string
	for _, item := range opf.Manifest.Items {
		if item.ID == coverID || item.ID == "cover-image" || item.ID == "cover" || strings.Contains(strings.ToLower(item.ID), "cover") {
			coverHref = item.Href
			break
		}
	}

	if coverHref != "" {
		// Resolve relative path to OPF path
		dir := filepath.Dir(opfPath)
		var fullCoverPath string
		if dir == "." {
			fullCoverPath = coverHref
		} else {
			fullCoverPath = filepath.Clean(filepath.Join(dir, coverHref))
		}
		fullCoverPath = strings.ReplaceAll(fullCoverPath, "\\", "/")

		for _, f := range r.File {
			if f.Name == fullCoverPath {
				imgRC, err := f.Open()
				if err == nil {
					meta.CoverData, _ = io.ReadAll(imgRC)
					imgRC.Close()
					meta.CoverExt = filepath.Ext(coverHref)
				}
				break
			}
		}
	}

	return meta, nil
}
