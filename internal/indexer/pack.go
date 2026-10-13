package indexer

import (
	"regexp"
	"strconv"
	"strings"
)

// Multi-book pack detection for release names (#2276).
//
// A Bindery download row carries exactly one BookID, and the importer
// computes exactly one destination folder from it. A release that is
// explicitly several books therefore has no correct outcome: every file in
// it lands in one book's folder. Nothing downstream can recover from that,
// so the release has to be refused at selection time.
//
// This is deliberately NOT the bundle detection in internal/metadata
// (bundle_titles.go). That one reads OpenLibrary catalogue titles, which are
// editorial prose; these are scene release names, which are punctuation
// soup. Its multi-title branch keys on spaced slashes, and the release that
// reported this issue carries "[ENG / M4B MP3]" — a tag separator, not an
// anthology. Reusing it here would have produced exactly the wrong rejection
// for exactly the release that motivated the work.
//
// What is matched, and why the boundaries sit where they do:
//
//   - An explicit numbered range after "book", "vol" or "volume": "Books 1-4",
//     "Book 1 to 4", "Vols. 1 & 2". Both numbers are capped at two digits so
//     "Book 1 - 2020 Edition" is not read as a range of 2020 books.
//   - A counted bundle: "4 Book Set", "3 Volume Collection".
//   - "box set" / "boxed set" / "boxset", "omnibus", and "complete series" or
//     "complete collection".
//
// What is deliberately NOT matched:
//
//   - "part". A single audiobook split into "Part 1-2" and "Part 3" is the
//     normal shape of a long recording, and is in fact the exact shape of the
//     download in #2275. Treating it as a pack would refuse ordinary
//     audiobooks.
//   - "trilogy". Real single books are routinely subtitled "Book III of the
//     Red Rising Trilogy" — again, the very release under discussion.
//   - A bare "collection", "anthology" or "series". Single books use all
//     three about themselves ("The Collected Stories", "Red Rising Series -
//     Book 1"). The shapes of those words that only a pack uses are matched
//     instead, see below.
//   - A bare numeric range with no preceding noun ("01-04"), which in a
//     release name is far more often a year span, a bitrate, or a chapter
//     count than a book range.
//
// Added for #3224, from the release names of one search for "Foundation":
//
//   - A counted bundle in brackets: "(11 Books)", "[03 Ebooks]". The count and
//     the noun sit inside brackets, which is where indexers put what the
//     release contains, not what the book is called.
//   - "Books Collection", "Collection Pack", "Series Pack" and a bracketed
//     "(Foundation Pack)": the word that makes the collection a product.
//   - "Saga eBooks", "Series eBooks": the plural noun after the series word
//     is the series' books, not one of them.
//   - "<name> series by <author>" with no book number anywhere in the name. A
//     release of one book names the book and gives it a number ("Red Rising
//     Series by Pierce Brown, Book 1"); a name that gives the series and the
//     author and nothing else is the whole series. "A Series of Unfortunate
//     Events" is a title, so "series of" never matches.
//   - A numeric range after "series" ("Series - Isaac Asimov 1-7"), unless
//     the number is a part or disc ("Series Part 1-2" is one split recording).
//     The range must be one or two digits on both sides and rising, so a year
//     span ("2014-2016") or an edition year ("1 - 2020") is not read as one.
var multiBookPackRe = regexp.MustCompile(`(?i)` +
	`\b(?:books?|vols?|volumes?)\.?\s*\d{1,2}\s*(?:[-–—+&]|to|thru|through)\s*\d{1,2}\b` +
	`|\b\d{1,2}\s*(?:books?|vol(?:ume)?s?)\s+(?:set|collection|bundle|pack|omnibus)\b` +
	`|\bbox(?:ed)?\s*set\b` +
	`|\bomnibus\b` +
	`|\bcomplete\s+(?:series|collection|saga)\b` +
	`|[(\[]\s*\d{1,3}\s*(?:e-?books?|books?|novels?|volumes?|titles?)\s*[)\]]` +
	`|\b(?:e-?)?books?\s+collection\b` +
	`|\b(?:book|series|collection|saga)\s+pack\b` +
	`|\bpack\s*[)\]]` +
	`|\b(?:series|saga)\s+e-?books\b`)

var (
	// seriesByRe matches "series by" ("series of" is a title, never a match).
	seriesByRe = regexp.MustCompile(`(?i)\bseries\s+by\b`)
	// bookNumberRe finds a release naming one numbered book or volume.
	bookNumberRe = regexp.MustCompile(`(?i)\b(?:book|vol(?:ume)?|bk|no|part|pt)\.?\s*#?\s*(?:\d+|[ivx]+|one|two|three|four|five|six|seven|eight|nine|ten)\b|#\s*\d+`)
	// seriesRangeRe finds a one or two digit range after the series word.
	seriesRangeRe = regexp.MustCompile(`(?i)\bseries\b([^\n]{0,60}?)\b(\d{1,2})\s*[-–—]\s*(\d{1,2})\b`)
	// partWordRe ends text that introduces a split recording or disc set.
	partWordRe = regexp.MustCompile(`(?i)\b(?:parts?|pts?|discs?|cds?|chapters?|ch)\.?\s*$`)
)

// seriesPackMarker reports the series wording of title that only a whole
// series is named with, or "" (#3224). See the notes above multiBookPackRe.
func seriesPackMarker(title string) string {
	if m := seriesRangeRe.FindStringSubmatchIndex(title); m != nil {
		lo, _ := strconv.Atoi(title[m[4]:m[5]])
		hi, _ := strconv.Atoi(title[m[6]:m[7]])
		if lo < hi && !partWordRe.MatchString(title[:m[4]]) {
			return strings.TrimSpace(title[m[0]:m[1]])
		}
	}
	if loc := seriesByRe.FindStringIndex(title); loc != nil && !bookNumberRe.MatchString(title) {
		return title[loc[0]:loc[1]]
	}
	return ""
}

// MultiBookPackMarker returns the substring of title that identifies it as a
// pack of several books, or "" when it names a single book. The matched text
// is returned rather than a bool so callers can tell the user which words
// they are being judged on.
func MultiBookPackMarker(title string) string {
	if m := multiBookPackRe.FindString(title); m != "" {
		return m
	}
	return seriesPackMarker(title)
}
