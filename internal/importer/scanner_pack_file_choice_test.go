package importer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vavallee/bindery/internal/models"
)

// seriesDownloadFixture seeds the ebook "Foundation" and a download for it,
// returning the scanner, the download and a directory the caller fills.
func seriesDownloadFixture(t *testing.T) (*Scanner, *models.Download, string, func() []models.BookFile) {
	t.Helper()
	s, dl, _, bookRepo, ctx := dataLossFixture(t, t.TempDir(), "copy")
	book, err := bookRepo.GetByID(ctx, *dl.BookID)
	if err != nil {
		t.Fatal(err)
	}
	book.Title = "Foundation"
	if err := bookRepo.Update(ctx, book); err != nil {
		t.Fatal(err)
	}
	dl.Title = "Foundation series by Isaac Asimov EPUB"
	files := func() []models.BookFile {
		got, err := bookRepo.ListBookFiles(ctx, book.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	return s, dl, t.TempDir(), files
}

// writeEbook writes an EPUB-shaped file large enough to pass the 4 KiB floor,
// tagged with marker so the test can tell which source landed.
func writeEbook(t *testing.T, path, marker string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	body := append(epubBytes(), bytes.Repeat([]byte(marker), 1024)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestImport_SeriesDownloadPlacesTheMatchingBook is #3224. A download holding
// a whole series gives every EPUB the same destination, and the first file in
// walk order used to take it: "Forward the Foundation (4048)/" sorts before
// "Foundation (4047)/", so the book Foundation was filed with the wrong novel.
func TestImport_SeriesDownloadPlacesTheMatchingBook(t *testing.T) {
	s, dl, downloadPath, files := seriesDownloadFixture(t)
	writeEbook(t, filepath.Join(downloadPath, "Forward the Foundation (4048)", "Forward the Foundation - Isaac Asimov.epub"), "forward")
	writeEbook(t, filepath.Join(downloadPath, "Foundation (4047)", "Foundation - Isaac Asimov.epub"), "foundation")
	writeEbook(t, filepath.Join(downloadPath, "Foundation and Empire (4049)", "Foundation and Empire - Isaac Asimov.epub"), "empire")

	s.tryImportInternal(t.Context(), dl, downloadPath, "", "", "", nil, nil)

	got := files()
	if len(got) != 1 {
		t.Fatalf("book files = %d, want 1: %+v", len(got), got)
	}
	data, err := os.ReadFile(got[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("foundation")) || bytes.Contains(data, []byte("forward")) {
		t.Errorf("the library file is not the Foundation EPUB from the download")
	}
}

// TestImport_SkipsFilesTooSmallToBeABook pins the #2944 floor on the
// post-download path: a release's link file under 4 KiB is not imported as a
// second format of the book beside the real EPUB.
func TestImport_SkipsFilesTooSmallToBeABook(t *testing.T) {
	s, dl, downloadPath, files := seriesDownloadFixture(t)
	writeEbook(t, filepath.Join(downloadPath, "Foundation - Isaac Asimov.epub"), "foundation")
	spam := []byte(strings.Repeat("www.example.online - free ebooks ", 7))
	if err := os.WriteFile(filepath.Join(downloadPath, "www.example.online.txt"), spam, 0o644); err != nil {
		t.Fatal(err)
	}

	s.tryImportInternal(t.Context(), dl, downloadPath, "", "", "", nil, nil)

	got := files()
	if len(got) != 1 || filepath.Ext(got[0].Path) != ".epub" {
		t.Fatalf("book files = %+v, want only the epub", got)
	}
}
