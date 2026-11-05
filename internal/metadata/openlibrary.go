package metadata

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

const defaultOpenLibraryBase = "https://openlibrary.org"

type olSearchResponse struct {
	Docs []olSearchDoc `json:"docs"`
}

type olSearchDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	AuthorKey        []string `json:"author_key"`
	FirstPublishYear int      `json:"first_publish_year"`
	CoverI           int      `json:"cover_i"`
	Isbn             []string `json:"isbn"`
	Language         []string `json:"language"`
	Publisher        []string `json:"publisher"`
	NumberOfPages    int      `json:"number_of_pages_median"`
	Subject          []string `json:"subject"`
}

type olWork struct {
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description any             `json:"description"`
	Covers      []int           `json:"covers"`
	Subjects    []string        `json:"subjects"`
	Authors     []olWorkAuthor  `json:"authors"`
	FirstPublishDate string     `json:"first_publish_date"`
}

type olWorkAuthor struct {
	Author struct {
		Key string `json:"key"`
	} `json:"author"`
}

type olAuthor struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Bio         any    `json:"bio"`
	Photos      []int  `json:"photos"`
	BirthDate   string `json:"birth_date"`
	DeathDate   string `json:"death_date"`
}

type olEdition struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	Isbn13        []string `json:"isbn_13"`
	Isbn10        []string `json:"isbn_10"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	NumberOfPages int      `json:"number_of_pages"`
	Languages     []struct {
		Key string `json:"key"`
	} `json:"languages"`
	Covers []int `json:"covers"`
}

// SearchBooks looks up works on OpenLibrary.
func (s *Service) SearchBooks(ctx context.Context, term string) ([]domain.Book, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Book{}, nil
	}
	lower := strings.ToLower(term)
	if key, ok := parsePrefixedString(lower, "ol:", "openlibrary:"); ok {
		book, author, editions, err := s.GetBookByWorkKey(ctx, normalizeOLKey(key, "/works/"))
		if err != nil {
			if isNotFound(err) {
				return []domain.Book{}, nil
			}
			return nil, err
		}
		_ = author
		_ = editions
		return []domain.Book{book}, nil
	}
	if isbn := extractISBN(term); isbn != "" {
		book, _, editions, err := s.GetBookByISBN(ctx, isbn)
		if err != nil {
			if isNotFound(err) {
				return []domain.Book{}, nil
			}
			return nil, err
		}
		_ = editions
		return []domain.Book{book}, nil
	}

	u := fmt.Sprintf("%s/search.json?q=%s&limit=25", s.openLibraryBase(), url.QueryEscape(term))
	var result olSearchResponse
	if err := s.getJSON(ctx, u, &result); err != nil {
		return nil, fmt.Errorf("openlibrary search: %w", err)
	}
	out := make([]domain.Book, 0, len(result.Docs))
	for _, doc := range result.Docs {
		out = append(out, mapOLSearchDoc(doc))
	}
	return out, nil
}

// SearchAuthors looks up authors on OpenLibrary.
func (s *Service) SearchAuthors(ctx context.Context, term string) ([]domain.Author, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []domain.Author{}, nil
	}
	lower := strings.ToLower(term)
	if key, ok := parsePrefixedString(lower, "ol:", "openlibrary:"); ok {
		author, err := s.GetAuthorByOLKey(ctx, normalizeOLKey(key, "/authors/"))
		if err != nil {
			if isNotFound(err) {
				return []domain.Author{}, nil
			}
			return nil, err
		}
		return []domain.Author{author}, nil
	}
	u := fmt.Sprintf("%s/search/authors.json?q=%s&limit=25", s.openLibraryBase(), url.QueryEscape(term))
	var result struct {
		Docs []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"docs"`
	}
	if err := s.getJSON(ctx, u, &result); err != nil {
		return nil, fmt.Errorf("openlibrary author search: %w", err)
	}
	out := make([]domain.Author, 0, len(result.Docs))
	for _, d := range result.Docs {
		key := d.Key
		if !strings.HasPrefix(key, "/authors/") {
			key = "/authors/" + strings.TrimPrefix(key, "OL")
			if !strings.HasPrefix(d.Key, "/") {
				key = "/authors/" + d.Key
			} else {
				key = d.Key
			}
		}
		out = append(out, domain.Author{
			Title:           d.Name,
			SortTitle:       strings.ToLower(d.Name),
			ForeignAuthorID: key,
			TitleSlug:       slugify(d.Name),
			CleanTitle:      cleanTitle(d.Name),
			Status:          "continuing",
			Monitored:       true,
			Images:          []domain.MediaImage{},
			Genres:          []string{},
			Tags:            []int64{},
			Statistics:      map[string]any{},
		})
	}
	return out, nil
}

// GetAuthorByOLKey fetches an OpenLibrary author.
func (s *Service) GetAuthorByOLKey(ctx context.Context, key string) (domain.Author, error) {
	key = normalizeOLKey(key, "/authors/")
	u := fmt.Sprintf("%s%s.json", s.openLibraryBase(), key)
	var a olAuthor
	if err := s.getJSON(ctx, u, &a); err != nil {
		return domain.Author{}, err
	}
	if a.Key == "" && a.Name == "" {
		return domain.Author{}, errNotFound
	}
	return mapOLAuthor(a), nil
}

// GetBookByWorkKey fetches a work, primary author, and editions.
func (s *Service) GetBookByWorkKey(ctx context.Context, workKey string) (domain.Book, domain.Author, []domain.Edition, error) {
	workKey = normalizeOLKey(workKey, "/works/")
	u := fmt.Sprintf("%s%s.json", s.openLibraryBase(), workKey)
	var work olWork
	if err := s.getJSON(ctx, u, &work); err != nil {
		return domain.Book{}, domain.Author{}, nil, err
	}
	if work.Key == "" && work.Title == "" {
		return domain.Book{}, domain.Author{}, nil, errNotFound
	}
	book := mapOLWork(work)

	var author domain.Author
	if len(work.Authors) > 0 {
		akey := work.Authors[0].Author.Key
		if akey != "" {
			if a, err := s.GetAuthorByOLKey(ctx, akey); err == nil {
				author = a
			} else {
				author = domain.Author{
					Title:           "Unknown Author",
					ForeignAuthorID: normalizeOLKey(akey, "/authors/"),
					Monitored:       true,
					Images:          []domain.MediaImage{},
					Genres:          []string{},
					Tags:            []int64{},
					Statistics:      map[string]any{},
				}
			}
		}
	}
	if author.Title == "" {
		author = domain.Author{
			Title:      "Unknown Author",
			Monitored:  true,
			Images:     []domain.MediaImage{},
			Genres:     []string{},
			Tags:       []int64{},
			Statistics: map[string]any{},
		}
	}

	editions, _ := s.fetchWorkEditions(ctx, workKey, book.Title)
	return book, author, editions, nil
}

// GetBookByISBN resolves an ISBN to a work via OpenLibrary.
func (s *Service) GetBookByISBN(ctx context.Context, isbn string) (domain.Book, domain.Author, []domain.Edition, error) {
	isbn = strings.ReplaceAll(strings.TrimSpace(isbn), "-", "")
	u := fmt.Sprintf("%s/isbn/%s.json", s.openLibraryBase(), url.PathEscape(isbn))
	var ed olEdition
	if err := s.getJSON(ctx, u, &ed); err != nil {
		return domain.Book{}, domain.Author{}, nil, err
	}
	// Edition JSON often includes works link via nested structure — fall back to search.
	u2 := fmt.Sprintf("%s/search.json?isbn=%s&limit=1", s.openLibraryBase(), url.QueryEscape(isbn))
	var result olSearchResponse
	if err := s.getJSON(ctx, u2, &result); err != nil || len(result.Docs) == 0 {
		book := domain.Book{
			Title:         ed.Title,
			ForeignBookID: ed.Key,
			TitleSlug:     slugify(ed.Title),
			Monitored:     true,
			PageCount:     ed.NumberOfPages,
			Images:        olCoverImages(ed.Covers),
			Genres:        []string{},
		}
		edition := mapOLEdition(ed, "ebook")
		return book, domain.Author{Title: "Unknown Author", Monitored: true, Images: []domain.MediaImage{}, Genres: []string{}, Tags: []int64{}, Statistics: map[string]any{}}, []domain.Edition{edition}, nil
	}
	return s.GetBookByWorkKey(ctx, result.Docs[0].Key)
}

func (s *Service) fetchWorkEditions(ctx context.Context, workKey, fallbackTitle string) ([]domain.Edition, error) {
	u := fmt.Sprintf("%s%s/editions.json?limit=10", s.openLibraryBase(), workKey)
	var result struct {
		Entries []olEdition `json:"entries"`
	}
	if err := s.getJSON(ctx, u, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Edition, 0, len(result.Entries))
	for _, ed := range result.Entries {
		if ed.Title == "" {
			ed.Title = fallbackTitle
		}
		out = append(out, mapOLEdition(ed, "ebook"))
	}
	return out, nil
}

func (s *Service) openLibraryBase() string {
	if s.olBase != "" {
		return s.olBase
	}
	return defaultOpenLibraryBase
}

func mapOLSearchDoc(doc olSearchDoc) domain.Book {
	genres := doc.Subject
	if len(genres) > 8 {
		genres = genres[:8]
	}
	release := ""
	if doc.FirstPublishYear > 0 {
		release = strconv.Itoa(doc.FirstPublishYear)
	}
	images := []domain.MediaImage{}
	if doc.CoverI > 0 {
		remote := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI)
		images = append(images, domain.MediaImage{CoverType: "poster", URL: remote, RemoteURL: remote})
	}
	overview := ""
	if len(doc.AuthorName) > 0 {
		overview = "by " + strings.Join(doc.AuthorName, ", ")
	}
	return domain.Book{
		Title:         doc.Title,
		ForeignBookID: doc.Key,
		TitleSlug:     slugify(doc.Title),
		ReleaseDate:   release,
		Overview:      overview,
		Monitored:     true,
		PageCount:     doc.NumberOfPages,
		Images:        images,
		Genres:        genres,
	}
}

func mapOLWork(w olWork) domain.Book {
	overview := ""
	switch v := w.Description.(type) {
	case string:
		overview = v
	case map[string]any:
		if s, ok := v["value"].(string); ok {
			overview = s
		}
	}
	genres := w.Subjects
	if len(genres) > 12 {
		genres = genres[:12]
	}
	return domain.Book{
		Title:         w.Title,
		ForeignBookID: normalizeOLKey(w.Key, "/works/"),
		TitleSlug:     slugify(w.Title),
		ReleaseDate:   w.FirstPublishDate,
		Overview:      overview,
		Monitored:     true,
		Images:        olCoverImages(w.Covers),
		Genres:        genres,
	}
}

func mapOLAuthor(a olAuthor) domain.Author {
	overview := ""
	switch v := a.Bio.(type) {
	case string:
		overview = v
	case map[string]any:
		if s, ok := v["value"].(string); ok {
			overview = s
		}
	}
	images := []domain.MediaImage{}
	if len(a.Photos) > 0 && a.Photos[0] > 0 {
		remote := fmt.Sprintf("https://covers.openlibrary.org/a/id/%d-L.jpg", a.Photos[0])
		images = append(images, domain.MediaImage{CoverType: "poster", URL: remote, RemoteURL: remote})
	}
	status := "continuing"
	if a.DeathDate != "" {
		status = "ended"
	}
	name := a.Name
	key := normalizeOLKey(a.Key, "/authors/")
	return domain.Author{
		Title:           name,
		SortTitle:       strings.ToLower(name),
		Status:          status,
		Overview:        overview,
		ForeignAuthorID: key,
		TitleSlug:       slugify(name),
		CleanTitle:      cleanTitle(name),
		Monitored:       true,
		Images:          images,
		Genres:          []string{},
		Tags:            []int64{},
		Statistics:      map[string]any{},
	}
}

func mapOLEdition(ed olEdition, format string) domain.Edition {
	isbn13, isbn10 := "", ""
	if len(ed.Isbn13) > 0 {
		isbn13 = ed.Isbn13[0]
	}
	if len(ed.Isbn10) > 0 {
		isbn10 = ed.Isbn10[0]
	}
	publisher := ""
	if len(ed.Publishers) > 0 {
		publisher = ed.Publishers[0]
	}
	lang := "eng"
	if len(ed.Languages) > 0 {
		lang = strings.TrimPrefix(ed.Languages[0].Key, "/languages/")
	}
	title := ed.Title
	return domain.Edition{
		ForeignEditionID: normalizeOLKey(ed.Key, "/books/"),
		Title:            title,
		Format:           format,
		Isbn13:           isbn13,
		Isbn10:           isbn10,
		Language:         lang,
		PageCount:        ed.NumberOfPages,
		Monitored:        true,
		Publisher:        publisher,
		ReleaseDate:      ed.PublishDate,
		Images:           olCoverImages(ed.Covers),
	}
}

func olCoverImages(covers []int) []domain.MediaImage {
	if len(covers) == 0 || covers[0] <= 0 {
		return []domain.MediaImage{}
	}
	remote := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", covers[0])
	return []domain.MediaImage{{CoverType: "poster", URL: remote, RemoteURL: remote}}
}

func normalizeOLKey(key, prefix string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, prefix) {
		return key
	}
	if strings.HasPrefix(key, "/") {
		return key
	}
	return prefix + key
}

func extractISBN(term string) string {
	term = strings.TrimSpace(term)
	lower := strings.ToLower(term)
	for _, p := range []string{"isbn:", "isbn13:", "isbn10:"} {
		if strings.HasPrefix(lower, p) {
			term = strings.TrimSpace(term[len(p):])
			break
		}
	}
	cleaned := strings.ReplaceAll(term, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if len(cleaned) == 10 || len(cleaned) == 13 {
		for _, r := range cleaned {
			if r < '0' || r > '9' {
				if !(len(cleaned) == 10 && (r == 'X' || r == 'x')) {
					return ""
				}
			}
		}
		return cleaned
	}
	return ""
}
