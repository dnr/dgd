package main

import (
	"strings"
	"unique"
)

type ID unique.Handle[string]

func MakeID(s string) ID {
	return ID(unique.Make(s))
}

func (id ID) Zero() bool     { return id == zeroID }
func (id ID) String() string { return unique.Handle[string](id).Value() }

var zeroID ID

type Command struct {
	Cwd         string
	Args        []string
	Heredoc     string
	UsedCwd     bool
	UsedSubdirs map[string]struct{}
}

type Action struct {
	// filled in by Parser:
	Title    string
	Workdir  string
	Commands []*Command // list of commands

	// filled in by Build:
	ID      ID       // e.g. "b123"
	Outputs []string // files produced, relative to workdir
	Deps    []ID     // IDs of other actions this action depends on
}

type Graph struct {
	Actions     map[ID]*Action
	GoPkg       string
	ProjectRoot string
	DepRoot     string
	DepMap      map[string]string
}

func (a *Action) InWorkdir(s string) string {
	return strings.TrimPrefix(s, a.Workdir+"/")
}

func (c *Command) RecordSubdir(dir string) {
	c.UsedCwd = true
	if c.UsedSubdirs == nil {
		c.UsedSubdirs = make(map[string]struct{})
	}
	c.UsedSubdirs[dir] = struct{}{}
}
