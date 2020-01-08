package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

var (
	commit  = "unknown-commit"
	date    = "unknown-date"
	version = "unknown-version"
)

const (
	productName        = "makes"
	productDescription = "Interactively select make targets from a Makefile"
	maxSize            = 10
)

type flags struct{}

func main() {
	var (
		usage = flag.Usage
		flags = flags{}
	)
	flag.Usage = func() {
		fmt.Printf("%s (%s built from %s at %s)\n", productName, version, commit, date)
		fmt.Println(productDescription)
		usage()
	}
	flag.Parse()

	err := run(flags)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

}

type Target struct {
	Name    string
	Help    string
	IsPhony bool
	Updated time.Time
}

type RawTarget struct {
	Lines []string
	Help  string
}

func (t RawTarget) Name() string {
	if t.Lines == nil {
		return "# Not a target:"
	}
	if len(t.Lines) == 0 {
		return "# Not a target:"
	}
	parts := strings.Split(t.Lines[0], ":")
	return parts[0]
}

func (t RawTarget) IsPhony() bool {
	for _, l := range t.Lines {
		if strings.Contains(l, "Phony target (prerequisite of .PHONY)") {
			return true
		}
	}
	return false
}

func (t RawTarget) LastUpdate() (time.Time, error) {
	for _, l := range t.Lines {
		if strings.Contains(l, "Last modified") {
			parts := strings.Split(l, " ")
			date := fmt.Sprintf("%s %s", parts[len(parts)-2], parts[len(parts)-1])
			return time.ParseInLocation("2006-01-02 15:04:05", date, time.Local)
		}
	}
	return time.Time{}, nil
}

func run(f flags) error {
	started := time.Now()
	cmd := exec.Command("make", "-n", "-p")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	rtargets := []RawTarget{}
	version := ""

	buf := bytes.NewBuffer(out)
	scanner := bufio.NewScanner(buf)
	scanner.Split(bufio.ScanLines)

	if scanner.Scan() {
		version = strings.Replace(scanner.Text(), "# ", "", 1)
	}

	// skip to files section
	for scanner.Scan() {
		if scanner.Text() != "# Files" {
			continue
		}
		break
	}

	// skip to first target
	if !scanner.Scan() {
		return errors.New("unexpected EOF")
	}

	// read targets
	target := RawTarget{Lines: []string{}}
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			switch {
			case strings.HasPrefix(target.Name(), "#"):
			case strings.HasPrefix(target.Name(), ".PHONY"):
			default:
				rtargets = append(rtargets, target)
			}
			target = RawTarget{Lines: []string{}}
			continue
		}

		target.Lines = append(target.Lines, line)
	}

	file, err := os.Open("Makefile")
	if err != nil {
		return err
	}
	comments := map[string]string{}
	scanner = bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "##") {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				continue
			}
			comments[parts[0]] = parts[1]
		}
	}

	targets := []Target{}

	for _, t := range rtargets {
		name := t.Name()
		target := Target{
			Name:    name,
			IsPhony: t.IsPhony(),
		}
		if c, ok := comments[name]; ok {
			parts := strings.Split(c, "##")
			t.Help = strings.TrimSpace(parts[len(parts)-1])
			target.Help = t.Help
		}
		updated, err := t.LastUpdate()
		if err != nil {
			return err
		}
		target.Updated = updated
		targets = append(targets, target)
	}

	size := maxSize
	if len(targets) < size {
		size = len(targets)
	}

	templates := &promptui.SelectTemplates{
		Label:    "  {{ .Name }}",
		Active:   promptui.IconSelect + " {{ .Name }}",
		Inactive: "  {{ .Name | faint }}",
		Selected: promptui.IconGood + " {{ .Name }}",
		Details: `
----
Help: {{ .Help }}
{{ if .IsPhony }}Phony target{{else}}Last updated: {{if .Updated.IsZero }}never{{else}}{{.Updated}}{{end}}{{end}}`,
	}

	searcher := func(input string, index int) bool {
		pepper := targets[index]
		name := strings.Replace(strings.ToLower(pepper.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:             "Select a make target",
		Items:             targets,
		Templates:         templates,
		Searcher:          searcher,
		Size:              size,
		StartInSearchMode: true,
	}

	fmt.Printf("\n%s (duration=%s)\n\n", version, time.Since(started))
	n, _, err := prompt.Run()

	if err != nil {
		return err
	}

	fmt.Printf("Running %q ...\n", "make "+targets[n].Name)

	cmd = exec.Command("make", targets[n].Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
