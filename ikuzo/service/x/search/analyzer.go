package search

import "strings"

const (
	trimCharacters = ".,;:[]()?"
)

// Analyzer is the default analyzer for Search actions.
// It folds unicode to ASCII characters and lowercases them all.
//
// The goal is to have this analyzer behave similarly to the ElasticSearch
// Analyzer that Ikuzo comes preconfigured with.
type Analyzer struct{}

func (a *Analyzer) Transform(text string) string {
	return strings.Trim(
		strings.ToLower(
			LuceneASCIIFolding(text),
		),
		trimCharacters,
	)
}