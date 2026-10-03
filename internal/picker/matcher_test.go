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
	{Key: "work/login"},
	{Key: "work/logout"},
	{Key: "work/signup"},
	{Key: "release/1.0"},
	{Key: "hotfix/1.0.1"},
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name, query, want string
	}{
		{"exact", "work/login", "work/login"},
		{"exact beats substring", "work/logout", "work/logout"},
		{"case-insensitive", "WORK/SIGNUP", "work/signup"},
		{"unique substring", "sign", "work/signup"},
		{"ambiguous substring", "log", ""},
		{"fuzzy unique", "wsu", "work/signup"},
		{"fuzzy ambiguous", "wl", ""},
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
	rows := []Row{{Key: "work/a"}, {Key: "work/ab"}}
	if r, ok := Resolve(rows, "work/a"); !ok || r.Key != "work/a" {
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
