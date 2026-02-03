package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func Build(actions []*Action, gopkg, initialRoot, depRoot string) (*Graph, error) {
	g := &Graph{
		Actions:     make(map[ID]*Action),
		GoPkg:       gopkg,
		ProjectRoot: initialRoot,
		DepRoot:     depRoot,
		DepMap:      loadDepMap(depRoot),
	}

	usedOutputs := make(map[ID]map[string]struct{})

	// First pass: identify ID and outputs
	for _, a := range actions {
		if rest, found := strings.CutPrefix(a.Workdir, "$WORK/"); found {
			rest = strings.ReplaceAll(rest, "/exe", "-bin")
			rest = strings.ReplaceAll(rest, "/", "-")
			a.ID = MakeID(rest)
		} else {
			log.Fatalln("bad workdir", a.Workdir)
		}

		for _, cmd := range a.Commands {
			for i, arg := range cmd.Args {
				out := ""
				buildidIdx := 0
				if (arg == "-o" || arg == ">") && i+1 < len(cmd.Args) {
					out = cmd.Args[i+1]
				} else if arg[0] == '>' && len(arg) > 1 {
					out = arg[1:]
				} else if arg == "-buildid" {
					buildidIdx = i + 1
				}
				if rest, found := strings.CutPrefix(out, a.Workdir+"/"); found {
					a.Outputs = append(a.Outputs, rest)
				}

				// heuristics to see if this arg is an input or output in the current directory
				if i == 0 || i == buildidIdx || arg[0] == '-' || arg[0] == '/' || arg[0] == '$' {
					// not a filename or absolute
				} else if strings.HasPrefix(arg, "./") {
					// definitely in current dir
					cmd.UsedCwd = true
					// record all subdirs used
					cmd.RecordSubdir(filepath.Dir(arg))
				} else {
					// this isn't actually needed with go 1.25 build output, everything uses
					// "./", but for completeness:
					_, err := os.Stat(filepath.Join(cmd.Cwd, arg))
					if err == nil {
						cmd.UsedCwd = true
						cmd.RecordSubdir(filepath.Dir(arg))
					}
				}
			}
			if cmd.Args[0] == "mv" {
				a.Outputs = slices.DeleteFunc(a.Outputs, func(s string) bool { return s == a.InWorkdir(cmd.Args[1]) })
				a.Outputs = append(a.Outputs, a.InWorkdir(cmd.Args[2]))
			}
		}

		if _, ok := g.Actions[a.ID]; ok {
			log.Fatalln("duplicate action id", a.ID.String())
		}
		g.Actions[a.ID] = a
	}

	// Second pass: identify dependencies
	for _, a := range g.Actions {
		depSet := make(map[ID]struct{})

		checkDep := func(arg string) {
			rest, found := strings.CutPrefix(arg, "$WORK/")
			if !found {
				return
			}
			dir, file := filepath.Split(rest)
			dirForID := strings.ReplaceAll(strings.TrimRight(dir, "/"), "/", "-")
			depID := MakeID(dirForID)
			if depID == a.ID {
				return
			}
			_, ok := g.Actions[depID]
			if !ok {
				return
			}

			depSet[depID] = struct{}{}
			if uo, ok := usedOutputs[depID]; ok {
				uo[file] = struct{}{}
			} else {
				usedOutputs[depID] = map[string]struct{}{file: struct{}{}}
			}
		}

		for _, cmd := range a.Commands {
			if len(cmd.Args) == 0 || cmd.Args[0] == "mkdir" || cmd.Args[0] == "rm" {
				continue
			}

			for _, arg := range cmd.Args {
				checkDep(arg)
			}

			// Parse importcfg
			if cmd.Heredoc != "" && strings.Contains(strings.Join(cmd.Args, " "), "importcfg") {
				for line := range strings.Lines(cmd.Heredoc) {
					if strings.HasPrefix(line, "packagefile ") {
						if _, after, found := strings.Cut(line, "="); found {
							checkDep(strings.TrimRight(after, "\n"))
						}
					}
				}
			}
		}

		for depID := range depSet {
			a.Deps = append(a.Deps, depID)
		}
	}

	// Third pass: trim unused outputs
	for _, a := range g.Actions {
		if strings.HasSuffix(a.ID.String(), "-bin") {
			// retain all outputs
			continue
		}
		uo := usedOutputs[a.ID]
		a.Outputs = slices.DeleteFunc(a.Outputs, func(s string) bool {
			_, ok := uo[s]
			return !ok
		})
	}

	return g, nil
}

func loadDepMap(depRoot string) map[string]string {
	if depRoot == "" {
		return nil
	}

	out := make(map[string]string)
	err := filepath.WalkDir(depRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.Type() != fs.ModeSymlink {
			return nil
		}
		rel := path[len(depRoot)+1:]
		if !strings.HasPrefix(rel, "pkg/mod/") {
			return nil
		} else if rel == "pkg/mod/cache" {
			return nil
		}
		mod := rel[len("pkg/mod/"):]
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		out[mod] = target
		return nil
	})
	if err != nil {
		log.Fatalln("walking deproot error", err)
	}
	return out
}
