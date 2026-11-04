package native

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediacover"
)

func (s *Server) registerMusicBooksRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/artists", s.authSvc.RequirePermission(domain.PermMusicRead, s.handleListArtists))
	mux.HandleFunc("POST /api/v1/artists", s.authSvc.RequirePermission(domain.PermMusicWrite, s.handleCreateArtist))
	mux.HandleFunc("DELETE /api/v1/artists/{id}", s.authSvc.RequirePermission(domain.PermMusicDelete, s.handleDeleteArtist))
	mux.HandleFunc("GET /api/v1/artists/lookup", s.authSvc.RequirePermission(domain.PermMusicRead, s.handleLookupArtists))
	mux.HandleFunc("GET /api/v1/albums", s.authSvc.RequirePermission(domain.PermMusicRead, s.handleListAlbums))
	mux.HandleFunc("GET /api/v1/tracks", s.authSvc.RequirePermission(domain.PermMusicRead, s.handleListTracks))

	mux.HandleFunc("GET /api/v1/authors", s.authSvc.RequirePermission(domain.PermBooksRead, s.handleListAuthors))
	mux.HandleFunc("POST /api/v1/authors", s.authSvc.RequirePermission(domain.PermBooksWrite, s.handleCreateAuthor))
	mux.HandleFunc("DELETE /api/v1/authors/{id}", s.authSvc.RequirePermission(domain.PermBooksDelete, s.handleDeleteAuthor))
	mux.HandleFunc("GET /api/v1/authors/lookup", s.authSvc.RequirePermission(domain.PermBooksRead, s.handleLookupAuthors))
	mux.HandleFunc("GET /api/v1/books", s.authSvc.RequirePermission(domain.PermBooksRead, s.handleListBooks))
	mux.HandleFunc("POST /api/v1/books", s.authSvc.RequirePermission(domain.PermBooksWrite, s.handleCreateBook))
	mux.HandleFunc("DELETE /api/v1/books/{id}", s.authSvc.RequirePermission(domain.PermBooksDelete, s.handleDeleteBook))
	mux.HandleFunc("GET /api/v1/books/lookup", s.authSvc.RequirePermission(domain.PermBooksRead, s.handleLookupBooks))
	mux.HandleFunc("GET /api/v1/editions", s.authSvc.RequirePermission(domain.PermBooksRead, s.handleListEditions))
}

func (s *Server) handleListArtists(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListArtists()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateArtist(w http.ResponseWriter, r *http.Request) {
	var a domain.Artist
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateArtist(a)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.syncArtistDiscography(r, created)
	s.cacheArtistCover(created)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) syncArtistDiscography(r *http.Request, artist *domain.Artist) {
	if artist == nil || artist.ForeignArtistID == "" || s.meta == nil {
		return
	}
	_, items, err := s.meta.GetArtistByMBID(r.Context(), artist.ForeignArtistID)
	if err != nil || len(items) == 0 {
		return
	}
	_ = s.store.ReplaceArtistDiscography(artist.ID, items)
}

func (s *Server) cacheArtistCover(a *domain.Artist) {
	if a == nil || s.covers == nil {
		return
	}
	remote := mediacover.PosterRemoteURL(a.Images)
	if remote == "" {
		return
	}
	key := mediacover.CacheKeyMusic(a.ForeignArtistID, a.ID)
	local, err := s.covers.Ensure("music", key, a.ID, remote, a.ID)
	if err != nil || local == "" {
		return
	}
	for i := range a.Images {
		if strings.EqualFold(a.Images[i].CoverType, "poster") {
			a.Images[i].URL = local
		}
	}
	_, _ = s.store.UpdateArtist(*a)
}

func (s *Server) handleDeleteArtist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteArtist(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLookupArtists(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	if term == "" {
		term = r.URL.Query().Get("q")
	}
	items, err := s.meta.SearchArtists(r.Context(), term)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListAlbums(w http.ResponseWriter, r *http.Request) {
	artistID, _ := strconv.ParseInt(r.URL.Query().Get("artistId"), 10, 64)
	items, err := s.store.ListAlbums(artistID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListTracks(w http.ResponseWriter, r *http.Request) {
	albumID, _ := strconv.ParseInt(r.URL.Query().Get("albumId"), 10, 64)
	artistID, _ := strconv.ParseInt(r.URL.Query().Get("artistId"), 10, 64)
	items, err := s.store.ListTracks(albumID, artistID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListAuthors(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListAuthors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateAuthor(w http.ResponseWriter, r *http.Request) {
	var a domain.Author
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateAuthor(a)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteAuthor(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteAuthor(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLookupAuthors(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	if term == "" {
		term = r.URL.Query().Get("q")
	}
	items, err := s.meta.SearchAuthors(r.Context(), term)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request) {
	authorID, _ := strconv.ParseInt(r.URL.Query().Get("authorId"), 10, 64)
	items, err := s.store.ListBooks(authorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// createBookRequest adds a book (+ author upsert) and a monitored edition for ebook or audiobook.
type createBookRequest struct {
	domain.Book
	Author           domain.Author `json:"author"`
	Format           string        `json:"format"` // ebook | audiobook
	QualityProfileID int64         `json:"qualityProfileId"`
	RootFolderPath   string        `json:"rootFolderPath"`
	Monitored        *bool         `json:"monitored"`
}

func (s *Server) handleCreateBook(w http.ResponseWriter, r *http.Request) {
	var req createBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = "ebook"
	}
	if format != "ebook" && format != "audiobook" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("format must be ebook or audiobook"))
		return
	}

	author := req.Author
	if author.Title == "" && req.Book.Title != "" {
		author.Title = "Unknown Author"
	}
	var authorID int64
	if author.ForeignAuthorID != "" {
		if existing, err := s.store.GetAuthorByForeignID(author.ForeignAuthorID); err == nil && existing != nil {
			authorID = existing.ID
		}
	}
	if authorID == 0 {
		if req.RootFolderPath != "" {
			author.RootFolderPath = req.RootFolderPath
		} else if format == "audiobook" {
			author.RootFolderPath = "/data/Audiobooks"
		}
		createdAuthor, err := s.store.CreateAuthor(author)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		authorID = createdAuthor.ID
	}

	book := req.Book
	book.AuthorID = authorID
	if req.Monitored != nil {
		book.Monitored = *req.Monitored
	} else {
		book.Monitored = true
	}
	createdBook, err := s.store.CreateBook(book)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	root := req.RootFolderPath
	if root == "" {
		mt := format
		if folders, err := s.store.ListRootFolders(mt); err == nil {
			for _, f := range folders {
				root = f.Path
				break
			}
		}
		if root == "" {
			if format == "audiobook" {
				root = "/data/Audiobooks"
			} else {
				root = "/data/Books"
			}
		}
	}
	qp := req.QualityProfileID
	if qp == 0 {
		qp = 1
	}
	editionPath := strings.TrimRight(root, "/") + "/" + createdBook.Title
	edition := domain.Edition{
		BookID:           createdBook.ID,
		AuthorID:         authorID,
		ForeignEditionID: createdBook.ForeignBookID + "-" + format,
		Title:            createdBook.Title,
		Format:           format,
		Monitored:        true,
		Path:             editionPath,
		QualityProfileID: qp,
		RootFolderPath:   root,
		Images:           createdBook.Images,
		ReleaseDate:      createdBook.ReleaseDate,
		PageCount:        createdBook.PageCount,
	}
	createdEd, err := s.store.CreateEdition(edition)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"book":    createdBook,
		"edition": createdEd,
		"authorId": authorID,
	})
}

func (s *Server) handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteBook(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLookupBooks(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	if term == "" {
		term = r.URL.Query().Get("q")
	}
	items, err := s.meta.SearchBooks(r.Context(), term)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListEditions(w http.ResponseWriter, r *http.Request) {
	bookID, _ := strconv.ParseInt(r.URL.Query().Get("bookId"), 10, 64)
	format := r.URL.Query().Get("format")
	items, err := s.store.ListEditions(bookID, format)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
