package importer

import (
	"slices"

	"github.com/vavallee/bindery/internal/models"
	"github.com/vavallee/bindery/internal/textutil"
)

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
