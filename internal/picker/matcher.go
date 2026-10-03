package picker

import (
	"slices"
	"strings"
)

// Match tiers, best first.
const (
	tierExact = iota + 1
	tierSubstring
	tierFuzzy
)

var tiers = []int{tierExact, tierSubstring, tierFuzzy}

// Resolve returns the only row a query selects, without any UI. Matching is case-insensitive and
// tiered: exact, then substring, then fuzzy (the letters in order). The first tier with exactly one
// hit wins; a tier with several hits, or no hits at all, resolves nothing.
func Resolve(rows []Row, query string) (Row, bool) {
	q := strings.ToLower(query)
	if q == "" {
		return Row{}, false
	}
	return firstDecisive(rows, q, tiers)
}

// firstDecisive walks the tiers: one hit decides, several hits stop the search, none moves on.
func firstDecisive(rows []Row, q string, tiers []int) (Row, bool) {
	if len(tiers) == 0 {
		return Row{}, false
	}
	switch hits := rowsAt(rows, q, tiers[0]); len(hits) {
	case 0:
		return firstDecisive(rows, q, tiers[1:])
	case 1:
		return hits[0], true
	}
	return Row{}, false
}

func rowsAt(rows []Row, q string, tier int) []Row {
	return keep(rows, func(r Row) bool { return matchesAt(r, q, tier) })
}

// matchesAt reports whether the row's key or alt matches q at exactly this tier's rule.
func matchesAt(r Row, q string, tier int) bool {
	for _, s := range []string{r.Key, r.Alt} {
		if s != "" && tierRule(tier)(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

func tierRule(tier int) func(s, q string) bool {
	switch tier {
	case tierExact:
		return func(s, q string) bool { return s == q }
	case tierSubstring:
		return strings.Contains
	}
	return isSubsequence
}

// isSubsequence reports whether the runes of q appear in s in order.
func isSubsequence(s, q string) bool {
	return subseq([]rune(s), []rune(q))
}

func subseq(s, q []rune) bool {
	switch {
	case len(q) == 0:
		return true
	case len(s) == 0:
		return false
	case s[0] == q[0]:
		return subseq(s[1:], q[1:])
	}
	return subseq(s[1:], q)
}

// bestTier is the first tier at which the row matches, or 0 for no match.
func bestTier(r Row, q string) int {
	for _, t := range tiers {
		if matchesAt(r, q, t) {
			return t
		}
	}
	return 0
}

// Filter returns the rows to display for a query: every row that matches at any tier, ordered by
// best tier, then by original position. An empty query keeps all rows as they are.
func Filter(rows []Row, query string) []Row {
	q := strings.ToLower(query)
	if q == "" {
		return slices.Clone(rows)
	}
	matched := keep(rows, func(r Row) bool { return bestTier(r, q) > 0 })
	slices.SortStableFunc(matched, func(a, b Row) int { return bestTier(a, q) - bestTier(b, q) })
	return matched
}

func keep[T any](xs []T, pred func(T) bool) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if pred(x) {
			out = append(out, x)
		}
	}
	return out
}
