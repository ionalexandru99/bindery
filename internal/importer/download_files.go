package importer

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/vavallee/bindery/internal/models"
	"github.com/vavallee/bindery/internal/textutil"
)

// dropImplausibleEbooks removes the ebook files of a finished download that
// are too small to be a book (#2944) when the download also holds one that is
// not, so a release's link or notes file is never imported as another format
// of the book (#3224). A download holding nothing larger is left as it is.
// Manual import does not call it: that is the override for a file this small
// that really is the book.
//
// Only a file inside downloadPath is measured, through an fs.FS rooted there,
// so the stat cannot leave the download folder whatever the path holds. A
// file outside it is kept, as it was before this check existed.
func dropImplausibleEbooks(downloadPath string, files []string) []string {
	root := filepath.Clean(downloadPath)
	download := os.DirFS(root)
	tooSmall := make(map[string]bool)
	for _, path := range files {
		rel, err := filepath.Rel(root, filepath.Clean(path))
		if err != nil {
			continue
		}
		name := filepath.ToSlash(rel)
		if !fs.ValidPath(name) {
			continue
		}
		if info, err := fs.Stat(download, name); err == nil && TooSmallToBeABook(path, info.Size()) {
			tooSmall[path] = true
		}
	}
	if len(tooSmall) == 0 || len(tooSmall) == len(files) {
		return files
	}
	kept := make([]string, 0, len(files)-len(tooSmall))
	for _, path := range files {
		if tooSmall[path] {
			slog.Info("skipping a file too small to be a book", "file", path)
			continue
		}
		kept = append(kept, path)
	}
	return kept
}

// orderByBookTitle puts the files whose title is closest to the book's first.
// Every ebook file of one format shares a destination, and the import loop
// places the first one and counts the rest as already imported, so in a
// download holding a whole series the matching book has to lead (#3224). The
// sort is stable: files that score the same keep their walk order.
func orderByBookTitle(book *models.Book, files []string) []string {
	if book == nil || len(files) < 2 {
		return files
	}
	want := normalizeTitle(book.Title)
	scores := make(map[string]float64, len(files))
	for _, f := range files {
		scores[f] = fileTitleScore(want, f)
	}
	ordered := slices.Clone(files)
	slices.SortStableFunc(ordered, func(a, b string) int {
		switch {
		case scores[a] > scores[b]:
			return -1
		case scores[a] < scores[b]:
			return 1
		}
		return 0
	})
	return ordered
}

// fileTitleScore is the closer of the file name's title and the EPUB's
// dc:title to the book title, already normalised as want.
func fileTitleScore(want, path string) float64 {
	score := textutil.JaroWinkler(want, normalizeTitle(ParseFilename(path).Title))
	if IsEpubFile(path) {
		if meta, err := ReadEpubMetadata(path); err == nil && meta.Title != "" {
			score = max(score, textutil.JaroWinkler(want, normalizeTitle(meta.Title)))
		}
	}
	return score
}
