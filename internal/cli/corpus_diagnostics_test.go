package cli

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/prepaser/erafmt/internal/format"
)

type corpusCounts struct {
	Files, Read, Decoded, Verified, Changed, Unchanged, Partial, SyntaxErrors, EncodingErrors, ReadErrors int
}

type corpusGameReport struct {
	Name      string
	NameBytes string `json:",omitempty"`
	Counts    corpusCounts
}

type corpusReadFailure struct {
	Path      string
	PathBytes string `json:",omitempty"`
	Error     string
}

type corpusDiagnosticRepresentative struct {
	Path      string
	PathBytes string `json:",omitempty"`
	Line      int
	Column    int
	Message   string
}

type corpusDiagnosticReason struct {
	Reason         string
	Command        string
	Count          int
	Representative corpusDiagnosticRepresentative
}

const (
	corpusMaxDiagnosticCategories = 64
	corpusMaxDiagnosticReasons    = 32
	corpusMaxDiagnosticReason     = 256
	corpusMaxDiagnosticMessage    = 512
)

type corpusDiagnosticCategoryKey struct {
	Severity format.Severity
	Code     string
}

type corpusDiagnosticReasonKey struct {
	Reason  string
	Command string
}

type corpusDiagnosticCategory struct {
	Severity                 format.Severity
	Code                     string
	Count                    int
	Representative           corpusDiagnosticRepresentative
	Reasons                  []corpusDiagnosticReason
	ReasonOccurrencesOmitted int
}

type corpusDiagnosticCategoryAccumulator struct {
	corpusDiagnosticCategory
	reasons map[corpusDiagnosticReasonKey]*corpusDiagnosticReason
}

type corpusDiagnosticAccumulator struct {
	categories map[corpusDiagnosticCategoryKey]*corpusDiagnosticCategoryAccumulator
	omitted    int
}

func (a *corpusDiagnosticAccumulator) add(rel string, diagnostic format.Diagnostic) {
	if a.categories == nil {
		a.categories = make(map[corpusDiagnosticCategoryKey]*corpusDiagnosticCategoryAccumulator)
	}
	categoryKey := corpusDiagnosticCategoryKey{Severity: diagnostic.Severity, Code: diagnostic.Code}
	category := a.categories[categoryKey]
	if category == nil {
		if len(a.categories) >= corpusMaxDiagnosticCategories {
			a.omitted++
			return
		}
		category = &corpusDiagnosticCategoryAccumulator{corpusDiagnosticCategory: corpusDiagnosticCategory{
			Severity: diagnostic.Severity, Code: diagnostic.Code, Representative: corpusDiagnosticRepresentative{
				Path: filepath.ToSlash(rel), PathBytes: jsonPathBytes(filepath.ToSlash(rel)), Line: diagnostic.Line, Column: diagnostic.Column, Message: corpusDiagnosticText(diagnostic.Message, corpusMaxDiagnosticMessage),
			},
		}, reasons: make(map[corpusDiagnosticReasonKey]*corpusDiagnosticReason)}
		a.categories[categoryKey] = category
	}
	category.Count++
	representative := corpusDiagnosticRepresentative{Path: filepath.ToSlash(rel), PathBytes: jsonPathBytes(filepath.ToSlash(rel)), Line: diagnostic.Line, Column: diagnostic.Column, Message: corpusDiagnosticText(diagnostic.Message, corpusMaxDiagnosticMessage)}
	if corpusDiagnosticRepresentativeLess(representative, category.Representative) {
		category.Representative = representative
	}
	reason, command := corpusDiagnosticReasonInfo(diagnostic.Message)
	reasonKey := corpusDiagnosticReasonKey{Reason: reason, Command: command}
	group := category.reasons[reasonKey]
	if group == nil {
		if len(category.reasons) >= corpusMaxDiagnosticReasons {
			category.ReasonOccurrencesOmitted++
			return
		}
		group = &corpusDiagnosticReason{Reason: reason, Command: command, Representative: representative}
		category.reasons[reasonKey] = group
	}
	group.Count++
	if corpusDiagnosticRepresentativeLess(representative, group.Representative) {
		group.Representative = representative
	}
}

func (a *corpusDiagnosticAccumulator) snapshot() ([]corpusDiagnosticCategory, int) {
	categories := make([]corpusDiagnosticCategory, 0, len(a.categories))
	for _, category := range a.categories {
		categoryCopy := category.corpusDiagnosticCategory
		categoryCopy.Reasons = make([]corpusDiagnosticReason, 0, len(category.reasons))
		for _, reason := range category.reasons {
			categoryCopy.Reasons = append(categoryCopy.Reasons, *reason)
		}
		sort.Slice(categoryCopy.Reasons, func(i, j int) bool {
			if categoryCopy.Reasons[i].Reason != categoryCopy.Reasons[j].Reason {
				return categoryCopy.Reasons[i].Reason < categoryCopy.Reasons[j].Reason
			}
			return categoryCopy.Reasons[i].Command < categoryCopy.Reasons[j].Command
		})
		categories = append(categories, categoryCopy)
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Severity != categories[j].Severity {
			return categories[i].Severity < categories[j].Severity
		}
		return categories[i].Code < categories[j].Code
	})
	return categories, a.omitted
}

func corpusDiagnosticRepresentativeLess(a, b corpusDiagnosticRepresentative) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.PathBytes != b.PathBytes {
		return a.PathBytes < b.PathBytes
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Column != b.Column {
		return a.Column < b.Column
	}
	return a.Message < b.Message
}

func corpusDiagnosticText(message string, limit int) string {
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) <= limit {
		return message
	}
	return string(runes[:limit]) + "…"
}

func corpusDiagnosticReasonInfo(message string) (reason, command string) {
	const prefix = "unsupported command "
	if strings.HasPrefix(message, prefix) {
		command = strings.TrimPrefix(message, prefix)
		reason = prefix[:len(prefix)-1]
		if i := strings.Index(command, " ("); i >= 0 && strings.HasSuffix(command, ")") {
			reason += " " + strings.Join(strings.Fields(command[i+1:]), " ")
			command = command[:i]
		}
		return corpusDiagnosticKeyText(reason, corpusMaxDiagnosticReason), corpusDiagnosticKeyText(command, 128)
	}
	reason = strings.Join(strings.Fields(message), " ")
	if prefix, line, ok := strings.Cut(reason, " before line "); ok && strings.HasPrefix(prefix, "unclosed ") {
		if _, err := strconv.Atoi(line); err == nil {
			reason = prefix + " before line <line>"
		}
	}
	return corpusDiagnosticKeyText(reason, corpusMaxDiagnosticReason), ""
}

func corpusDiagnosticKeyText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return fmt.Sprintf("%s… [sha256=%x]", string(runes[:limit]), sha256.Sum256([]byte(text)))
}
