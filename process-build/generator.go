package main

import (
	"fmt"
	"io"
	"iter"
	"log"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var cachedTools = []string{
	"pack",
}

func Generate(g *Graph, projectSrc string) string {
	var sb strings.Builder
	sb.Grow(256 << 10)
	sb.WriteString(`
{
  srcStr,
  goStr,
  envJson,
  pkgsPath,
}:
let

# turn string back into path so we can refer to subdirectories and let them be copied
# individually into the store for more accurate change tracking.
# unsafeDiscardStringContext is required in case srcStr is in the store already.
src = /. + "/${builtins.unsafeDiscardStringContext srcStr}";
go = builtins.storePath goStr;
env = builtins.fromJSON envJson;
pkgs = import pkgsPath { };

runCommand = if env.CGO_ENABLED == 1 then pkgs.runCommandCC else pkgs.runCommand;

cached_tools = pkgs.runCommand "dgd-cached-tools" { inherit env; } ''
  export GOTOOLCHAIN=local
  export CGO_ENABLED=0
  export GOCACHE=$TMPDIR/.gocache
  mkdir -p $out/bin
  for tool in ` + strings.Join(cachedTools, ` `) + `; do
    cp $(${go}/bin/go tool -n $tool) $out/bin/
  done
'';

`)

	for id := range toposort(g) {
		a := g.Actions[id]
		sb.WriteString(varName(id) + " = runCommand \"dgd-" + toNixName(a.Title) + "\"")
		sb.WriteString(" { inherit env; } ''\n")
		formatCommands(&sb, g, a)
		if len(a.Outputs) > 1 {
			sb.WriteString("  mkdir -p $out\n")
			for _, out := range a.Outputs {
				fmt.Fprintf(&sb, "  cp $TMPDIR/%s $out/%s\n", filepath.Base(out), filepath.Base(out))
			}
		} else if len(a.Outputs) == 1 {
			fmt.Fprintf(&sb, "  cp $TMPDIR/%s $out\n", filepath.Base(a.Outputs[0]))
		} else {
			sb.WriteString("  touch $out\n")
		}
		sb.WriteString("'';\n\n")
	}

	// TODO: always b001?
	sb.WriteString("in b001-bin\n")
	return sb.String()
}

func formatCommands(out io.Writer, g *Graph, a *Action) {
	w := func(s string) { fmt.Fprintf(out, "  %s\n", s) }

	cmds := a.Commands
	if cmds[0].Args[0] != "mkdir" {
		log.Fatalln("must start with mkdir")
	}
	// stdenv has already created a build dir for us
	workdir := strings.TrimRight(cmds[0].Args[1], "/")
	if workdir == "-p" {
		workdir = strings.TrimRight(cmds[0].Args[2], "/")
	}

	replace := func(s string) string {
		if rest, found := strings.CutPrefix(s, g.ProjectRoot); found {
			source := "src"
			if rest != "" {
				source += ` + "` + rest + `"`
			}
			// re-copy just this directory to the store, just regular files, so that we have
			// the narrowest-possible dependency on source changes.
			return fmt.Sprintf(
				`${builtins.path { path = %s; name = "dgd-source"; filter = p: t: t == "regular"; } }`,
				source)
		}
		s = strings.ReplaceAll(s, g.GoPkg, "${go}")
		s = strings.ReplaceAll(s, workdir, "$TMPDIR")
		s = replaceWorkPaths(s, g, a.ID)
		s = replaceDepPaths(s, g.DepRoot, g.DepMap)
		return s
	}

	w(`printf '\e[32m%s\e[0m\n' '` + a.Title + `'`)
	// w("set -x")
	// w("seed=41235")
	w("export GOTOOLCHAIN=local")
	w("export GOCACHE=$TMPDIR/.gocache")

	lastcwd := ""
	stripSubdir := ""

	for j, cmd := range cmds {
		if cmd.UsedCwd {
			cwd := cmd.Cwd
			strip := ""
			if len(cmd.UsedSubdirs) == 1 {
				for dir := range cmd.UsedSubdirs {
					if dir != "." {
						cwd = filepath.Join(cwd, dir)
						strip = dir
					}
				}
			}
			cwd = replace(cwd)
			if cwd != lastcwd {
				w("cd " + cwd)
				lastcwd = cwd
				stripSubdir = strip
			}
		}

		args := slices.Clone(cmd.Args)

		if args[0] == "mkdir" && j == 0 {
			continue
		} else if args[0] == "mv" {
			// only used for final binary?
			if !strings.Contains(args[2], "/") {
				args[2] = "$TMPDIR/" + args[2]
			}
		} else if args[0] == "rm" {
			// ignore
			continue
		} else if args[0] == "cd" {
			// ignore all cd, handled above
			continue
		} else if args[0] == "gcc" && args[len(args)-2] == "||" && args[len(args)-1] == "true" {
			// cgo builds do this to test features, I guess? but it's useless at this point
			continue
		} else if strings.HasSuffix(args[0], "go") && args[1] == "tool" && args[2] == "buildid" {
			// we're not using go's build cache
			// TODO: put some nix-derived build id in there? or fix it to use a cached binary
			continue
		} else if strings.HasSuffix(args[0], "go") && args[1] == "tool" && slices.Contains(cachedTools, args[2]) {
			// replace with call to cached version
			args[2] = "${cached_tools}/bin/" + args[2]
			args = args[2:]
		}

		parts := make([]string, 0, len(args))
		quoteNext, skipNext := false, false
		for i, arg := range args {
			if i == 0 && arg == "go" {
				arg = "${go}/bin/go"
			} else if arg == "-trimpath" || arg == "echo" {
				// TODO: this is really hacky
				quoteNext = true
			} else if quoteNext {
				arg = `"` + arg + `"`
				quoteNext = false
			} else if arg == "-buildid" {
				// these are changing and messing up our caching. TODO: figure out why?
				// I think it's that the absolute paths of the deps are changing
				skipNext = true
				continue
			} else if skipNext {
				skipNext = false
				continue
			} else if strings.HasPrefix(arg, "-ldflags=") {
				// TODO: this is even hackier
				arg = `'` + arg + `'`
			} else if stripSubdir != "" && filepath.Dir(arg) == stripSubdir {
				arg = filepath.Base(arg)
			}

			arg = replace(arg)
			parts = append(parts, arg)
		}

		res := strings.Join(parts, " ")
		if cmd.Heredoc != "" {
			heredoc := replaceWorkPaths(cmd.Heredoc, g, a.ID)
			w(res + " <<EOF")
			for hline := range strings.Lines(heredoc) {
				w(strings.TrimRight(hline, "\n"))
			}
			w("EOF")
		} else {
			w(res)
		}
	}
	return
}

var workPathRegex = regexp.MustCompile(`\$WORK/(b[0-9]+)/([a-zA-Z0-9,._-]*)`)

func replaceWorkPaths(input string, g *Graph, self ID) string {
	return workPathRegex.ReplaceAllStringFunc(input, func(m string) string {
		before, after, found := strings.Cut(m[6:], "/")
		if !found {
			log.Fatalln("workPathRegex", m)
		}
		id := MakeID(before)
		if id == self {
			if after == "" {
				return "$TMPDIR"
			}
			return "$TMPDIR/" + after
		}
		node := g.Actions[id]
		if after == "" || len(node.Outputs) < 2 {
			return `${` + varName(id) + `}`
		}
		return fmt.Sprintf("${%s}/%s", varName(id), after)
	})
}

const joinedDepName = "dgd-joined-deps" // note: must match name in nix code

var depPathRegex = regexp.MustCompile(`/nix/store/.{32}-` + joinedDepName + `/pkg/mod/.+@v[^/]+`)

func replaceDepPaths(input string, depRoot string, depMap map[string]string) string {
	return depPathRegex.ReplaceAllStringFunc(input, func(m string) string {
		if !strings.HasPrefix(m, depRoot) {
			panic("wrong dep root")
		}
		key := m[len("/nix/store/")+32+1+len(joinedDepName)+len("/pkg/mod/"):]
		sp := depMap[key]
		if sp == "" {
			log.Fatalf("dep %q not found in dep map", key)
		}
		return fmt.Sprintf(`${builtins.storePath "%s"}`, sp)
	})
}

func varName(id ID) string {
	s := id.String()
	if s[0] >= 'a' && s[0] <= 'z' {
		return s
	}
	return "b_" + s
}

func toNixName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}

func toposort(g *Graph) iter.Seq[ID] {
	left := maps.Clone(g.Actions)
	return func(yield func(ID) bool) {
		for len(left) > 0 {
			id := pickOne(left)

		inner:
			for _, dep := range left[id].Deps {
				if _, ok := left[dep]; ok {
					id = dep
					goto inner
				}
			}

			if !yield(id) {
				return
			}
			delete(left, id)
		}
	}
}

func pickOne[K comparable, V any](m map[K]V) (k K) {
	for k = range m {
		break
	}
	return
}
