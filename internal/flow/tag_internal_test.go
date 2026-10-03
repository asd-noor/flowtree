package flow

import (
	"slices"
	"testing"
)

func TestTagArgs(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		name string
		e    Env
		o    FinishOpts
		want []string
	}{
		{"message", Env{S: s}, FinishOpts{Kind: Release, Message: "m"}, []string{"tag", "-a", "-m", "m", "v1", "master"}},
		{"file", Env{S: s}, FinishOpts{Kind: Release, MessageFile: "f"}, []string{"tag", "-a", "-F", "f", "v1", "master"}},
		{"signed", Env{S: s}, FinishOpts{Kind: Release, Sign: true, Message: "m"}, []string{"tag", "-s", "-m", "m", "v1", "master"}},
		{"key wins", Env{S: s}, FinishOpts{Kind: Release, Sign: true, SignKey: "K", Message: "m"}, []string{"tag", "-u", "K", "-m", "m", "v1", "master"}},
		{"no tty default message", Env{S: s}, FinishOpts{Kind: Hotfix}, []string{"tag", "-a", "-m", "Hotfix 1.0", "v1", "master"}},
		{"tty lets git open the editor", Env{S: s, Interactive: true}, FinishOpts{Kind: Release}, []string{"tag", "-a", "v1", "master"}},
	}
	for _, c := range cases {
		if got := tagArgs(&c.e, c.o, "v1", "1.0"); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
