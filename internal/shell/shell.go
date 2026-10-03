// Package shell generates the snippet that `eval "$(flowtree shell-init)"` installs:
// a wrapper that follows the cd-file protocol, plus cobra's completion script.
package shell

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Supported shells, in the order they are listed in errors.
var Supported = []string{"bash", "zsh", "fish"}

const posixWrapper = `flowtree() {
  local f rc d
  f=$(mktemp) || return 1
  FLOWTREE_CDFILE=$f command flowtree "$@"; rc=$?
  if [ -s "$f" ]; then d=$(cat "$f"); [ -d "$d" ] && builtin cd "$d"; fi
  rm -f "$f"
  return $rc
}
`

const fishWrapper = `function flowtree
    set -l f (mktemp); or return 1
    begin
        set -lx FLOWTREE_CDFILE $f
        command flowtree $argv
    end
    set -l rc $status
    if test -s $f
        set -l d (cat $f)
        test -d "$d"; and builtin cd "$d"
    end
    rm -f $f
    return $rc
end
command flowtree completion fish | source
`

var scripts = map[string]string{
	"bash": "# eval \"$(flowtree shell-init bash)\"\n" + posixWrapper +
		"source <(command flowtree completion bash)\n",
	"zsh": "# eval \"$(flowtree shell-init zsh)\"\n" + posixWrapper +
		"if ! (( $+functions[compdef] )); then autoload -Uz compinit && compinit; fi\n" +
		"source <(command flowtree completion zsh)\n",
	"fish": "# flowtree shell-init fish | source\n" + fishWrapper,
}

// Init returns the script for the named shell.
func Init(name string) (string, error) {
	if s, ok := scripts[name]; ok {
		return s, nil
	}
	return "", fmt.Errorf("unsupported shell %q (supported: %s)", name, strings.Join(Supported, ", "))
}

// Detect names the shell from a $SHELL value such as /usr/bin/zsh.
func Detect(shellPath string) string {
	return filepath.Base(shellPath)
}
