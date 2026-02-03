package main

import (
	"bufio"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/google/shlex"
)

func Parse(r io.Reader, knownGoPkg string) ([]*Action, string, error) {
	scanner := bufio.NewScanner(r)
	var actions []*Action
	var currentAction *Action
	cwd := "[unknown]"
	title := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# ") {
			title = line[2:]
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// each mkdir is a new action
		if strings.HasPrefix(line, "mkdir -p ") {
			currentAction = &Action{
				Title:   title,
				Workdir: strings.TrimRight(line[9:], "/"),
			}
			if strings.Contains(line, "/exe") {
				currentAction.Title += "-bin"
			}
			actions = append(actions, currentAction)
		} else if strings.HasPrefix(line, "cd ") {
			cwd = line[3:]
		}

		if currentAction == nil {
			log.Fatalln("action did not start with mkdir", line)
		}

		// Handle heredoc (cat >file << 'EOF')
		if strings.Contains(line, "<<") {
			cmd, err := parseHeredoc(line, scanner)
			if err != nil {
				log.Fatalln("parseHeredoc error", err)
			}
			cmd.Cwd = cwd
			currentAction.Commands = append(currentAction.Commands, cmd)
			continue
		}

		args, err := shlex.Split(line)
		if err != nil {
			log.Fatalln("shlex error", err)
		}
		for _, arg := range args {
			if gopkg := getGoPkg(arg); gopkg != "" {
				if knownGoPkg == "" {
					knownGoPkg = gopkg
				}
				if knownGoPkg != "" && gopkg != knownGoPkg {
					log.Fatalln("mismatched go packages")
				}
			}
		}
		cmd := &Command{Cwd: cwd, Args: args}
		currentAction.Commands = append(currentAction.Commands, cmd)
	}

	return actions, knownGoPkg, scanner.Err()
}

func parseHeredoc(line string, scanner *bufio.Scanner) (*Command, error) {
	// e.g. cat >$WORK/b005/importcfg << 'EOF' # internal
	parts := strings.Split(line, "<<")
	cmdPart := strings.TrimSpace(parts[0])
	delimPart := strings.TrimSpace(parts[1])

	// Remove comments and quotes from delimiter
	delim := strings.Fields(delimPart)[0]
	delim = strings.Trim(delim, "'\"")

	args, err := shlex.Split(cmdPart)
	if err != nil {
		return nil, err
	}

	var content []string
	for scanner.Scan() {
		l := scanner.Text()
		if strings.TrimSpace(l) == delim {
			content = append(content, "")
			break
		}
		content = append(content, l)
	}

	return &Command{
		Args:    args,
		Heredoc: strings.Join(content, "\n"),
	}, nil
}

var goPkgRe = regexp.MustCompile(`/nix/store/[a-z0-9]{32}-go-[0-9.-]+`)

func getGoPkg(arg string) string {
	m := goPkgRe.FindStringSubmatch(arg)
	if len(m) > 0 {
		return m[0]
	}
	return ""
}
