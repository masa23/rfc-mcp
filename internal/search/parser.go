package search

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// RFCIndex represents the root element of rfc-index.xml
type RFCIndex struct {
	RFCs []RFCEntry `xml:"rfc-entry"`
}

// RFCEntry represents a single RFC entry in the index
type RFCEntry struct {
	DocID   string   `xml:"doc-id"`
	Title   string   `xml:"title"`
	Status  string   `xml:"current-status"`
	Date    RFCDate  `xml:"date"`
	Authors []Author `xml:"author"`
}

type RFCDate struct {
	Year  string `xml:"year"`
	Month string `xml:"month"`
}

// Author represents an author of an RFC
type Author struct {
	Name string `xml:"name"`
}

// ParseRFCIndex parses the rfc-index.xml content using streaming XML decoder
func ParseRFCIndex(reader io.Reader) ([]RFCItem, error) {
	decoder := xml.NewDecoder(reader)

	var items []RFCItem

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading XML: %w", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "rfc-entry" {
				var entry RFCEntry
				if err := decoder.DecodeElement(&entry, &se); err != nil {
					return nil, fmt.Errorf("error decoding RFC entry: %w", err)
				}

				// Parse RFC number from doc-id (e.g., "RFC8259" -> 8259)
				number := parseRFCNumber(entry.DocID)

				item := RFCItem{
					Number: number,
					URL:    fmt.Sprintf("https://www.rfc-editor.org/rfc/rfc%d.txt", number),
				}

				item.Title = strings.TrimSpace(entry.Title)
				item.Status = strings.TrimSpace(entry.Status)
				item.Date = formatRFCDate(entry.Date)

				items = append(items, item)
			}
		}
	}

	return items, nil
}

// SearchRFCs searches RFC items based on query terms
func SearchRFCs(items []RFCItem, query string, limit int) []RFCItem {
	terms := strings.Fields(strings.ToLower(query))

	type scoredItem struct {
		item  RFCItem
		score int
	}

	var scored []scoredItem

	for _, item := range items {
		score := 0

		title := strings.ToLower(item.Title)
		status := strings.ToLower(item.Status)

		for _, term := range terms {
			// For AND search, all terms must match
			foundInTitle := strings.Contains(title, term)
			foundInStatus := strings.Contains(status, term)
			foundInNumber := strings.Contains(fmt.Sprintf("%d", item.Number), term)

			if foundInTitle {
				score += 10
			} else if foundInStatus {
				score += 4
			} else if foundInNumber {
				score += 2
			} else {
				// If any term doesn't match, exclude this item
				score = 0
				break
			}
		}

		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Sort by score (descending), then RFC number (descending)
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].item.Number > scored[j].item.Number
	})

	// Apply limit
	result := make([]RFCItem, 0, len(scored))
	for i, s := range scored {
		if limit > 0 && i >= limit {
			break
		}
		result = append(result, s.item)
	}

	return result
}

var reRFCNumber = regexp.MustCompile(`\d+`)

func parseRFCNumber(docID string) int {
	m := reRFCNumber.FindString(docID)
	if m == "" {
		return 0
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0
	}
	return n
}

func formatRFCDate(d RFCDate) string {
	y := strings.TrimSpace(d.Year)
	m := strings.TrimSpace(d.Month)
	if y == "" {
		return ""
	}
	if m == "" {
		return y
	}
	// Month may be a name like "December".
	mm := normalizeMonth(m)
	if mm == "" {
		return y
	}
	return fmt.Sprintf("%s-%s", y, mm)
}

func normalizeMonth(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return ""
	}
	if len(s) <= 2 {
		// zero-pad
		if len(s) == 1 {
			return "0" + s
		}
		return s
	}
	monthMap := map[string]string{
		"january":   "01",
		"february":  "02",
		"march":     "03",
		"april":     "04",
		"may":       "05",
		"june":      "06",
		"july":      "07",
		"august":    "08",
		"september": "09",
		"october":   "10",
		"november":  "11",
		"december":  "12",
	}
	if v, ok := monthMap[s]; ok {
		return v
	}
	return ""
}
