package picker

import (
	"slices"
	"testing"
)

func keys(rows []Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key
	}
	return out
}

var sample = []Row{
	{Key: "@", Alt: "develop"},
	{Key: "feature/login"},
	{Key: "feature/logout"},
	{Key: "feature/signup"},
	{Key: "release/1.0"},
	{Key: "hotfix/1.0.1"},
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name, query, want string
	}{
		{"exact", "feature/login", "feature/login"},
		{"exact beats substring", "feature/logout", "feature/logout"},
		{"case-insensitive", "FEATURE/SIGNUP", "feature/signup"},
		{"unique substring", "sign", "feature/signup"},
		{"ambiguous substring", "log", ""},
		{"fuzzy unique", "fsu", "feature/signup"},
		{"fuzzy ambiguous", "fl", ""},
		{"alt matches", "develop", "@"},
		{"alt substring", "devel", "@"},
		{"no match", "zzz", ""},
		{"empty query resolves nothing", "", ""},
		{"ambiguous tier stops the search", "1.0", ""},
	}
	for _, c := range cases {
		row, ok := Resolve(sample, c.query)
		if c.want == "" && ok || c.want != "" && (!ok || row.Key != c.want) {
			t.Errorf("%s: Resolve(%q) = %q, %v; want %q", c.name, c.query, row.Key, ok, c.want)
		}
	}
}

func TestResolveExactWinsOverLongerSubstringHits(t *testing.T) {
	rows := []Row{{Key: "feature/a"}, {Key: "feature/ab"}}
	if r, ok := Resolve(rows, "feature/a"); !ok || r.Key != "feature/a" {
		t.Errorf("got %q %v", r.Key, ok)
	}
}

func TestFilterOrdersByTierThenPosition(t *testing.T) {
	rows := []Row{{Key: "x-release"}, {Key: "rel"}, {Key: "reXlease"}, {Key: "release/1"}, {Key: "other"}}
	got := keys(Filter(rows, "rel"))
	want := []string{"rel", "x-release", "release/1", "reXlease"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if all := Filter(rows, ""); len(all) != len(rows) {
		t.Errorf("empty query keeps all rows, got %d", len(all))
	}
	if none := Filter(rows, "qqq"); len(none) != 0 {
		t.Errorf("got %v", keys(none))
	}
}

func TestFilterDoesNotAliasInput(t *testing.T) {
	rows := []Row{{Key: "b"}, {Key: "a"}}
	_ = Filter(rows, "")
	rows2 := Filter(rows, "")
	rows2[0].Key = "changed"
	if rows[0].Key != "b" {
		t.Error("Filter must not share storage with its input")
	}
}

func TestIsSubsequence(t *testing.T) {
	for _, c := range [][2]string{{"abc", "ac"}, {"abc", "abc"}, {"héllo", "hlo"}, {"abc", ""}} {
		if !isSubsequence(c[0], c[1]) {
			t.Errorf("%q should contain %q in order", c[0], c[1])
		}
	}
	if isSubsequence("abc", "cb") || isSubsequence("", "a") {
		t.Error("order and emptiness matter")
	}
}
