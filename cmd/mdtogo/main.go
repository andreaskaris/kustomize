// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

// Package main generates cobra.Command go variables containing documentation read from .md files.
// Usage: mdtogo SOURCE_MD_DIR/ DEST_GO_DIR/ [--full=true] [--license=license.txt|none]
//
// The command will create a docs.go file under DEST_GO_DIR/ containing string variables to be
// used by cobra commands for documentation.The variable names are generated from the SOURCE_MD_DIR/
// file names, replacing '-' with '', title casing the filename, and dropping the extension.
// All *.md will be read from DEST_GO_DIR/, and a single DEST_GO_DIR/docs.go file is generated.
//
// Each .md document will be parsed as follows if no flags are provided:
//
//   ## cmd
//
//   This section will be parsed into a string variable for `Short`
//
//   ### Synopsis
//
//   This section will be parsed into a string variable for `Long`
//
//   ### Examples
//
//   This section will be parsed into a string variable for `Example`
//
// If --full=true is provided, the document will be parsed as follows:
//
//   ## cmd
//
//   All sections will be parsed into a Long string.
//
// Flags:
//   --full=true
//     Create a Long variable from the full .md files, rather than separate sections.
//   --license
//     Controls the license header added to the files.  Specify a path to a license file,
//     or "none" to skip adding a license.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var full bool
var licenseFile string

func main() {
	for _, a := range os.Args {
		if a == "--full=true" {
			full = true
		}
		if strings.HasPrefix(a, "--license=") {
			licenseFile = strings.ReplaceAll(a, "--license=", "")
		}
	}

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mdtogo SOURCE_MD_DIR/ DEST_GO_DIR/\n")
		os.Exit(1)
	}
	source := os.Args[1]
	dest := os.Args[2]

	files, err := os.ReadDir(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	var docs []doc
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".md" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(source, f.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		docs = append(docs, parse(f.Name(), string(b)))
	}

	var license string

	if licenseFile == "" {
		license = `// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0`
	} else if licenseFile == "none" {
		// no license -- maybe added by another tool
	} else {
		b, err := os.ReadFile(licenseFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		license = string(b)
	}

	out := []string{license, `
// Code generated by "mdtogo"; DO NOT EDIT.
package ` + filepath.Base(dest) + "\n"}

	for i := range docs {
		out = append(out, docs[i].String())
	}

	if _, err := os.Stat(dest); err != nil {
		_ = os.Mkdir(dest, 0700)
	}

	o := strings.Join(out, "\n")
	err = os.WriteFile(filepath.Join(dest, "docs.go"), []byte(o), 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func parse(name, value string) doc {
	name = strings.ReplaceAll(name, filepath.Ext(name), "")
	name = strings.Title(name)
	name = strings.ReplaceAll(name, "-", "")

	scanner := bufio.NewScanner(bytes.NewBufferString(value))

	var long, examples []string
	var short string
	var isLong, isExample, isIndent bool
	var doc doc

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") && short == "" {
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					continue
				}
				short = scanner.Text()
				break
			}
			continue
		}

		if !full {
			if strings.HasPrefix(line, "### Synopsis") {
				isLong = true
				isExample = false
				continue
			}

			if strings.HasPrefix(line, "### Examples") {
				isLong = false
				isExample = true
				continue
			}

			if strings.HasPrefix(line, "### ") {
				isLong = false
				isExample = false
				continue
			}
		}

		if strings.HasPrefix(line, "```") {
			isIndent = !isIndent
			continue
		}
		line = strings.ReplaceAll(line, "`", "` + \"`\" + `")
		if isIndent {
			line = "\t" + line
		}

		if isLong || full {
			long = append(long, line)
			continue
		}
		if isExample {
			examples = append(examples, line)
		}
	}

	doc.Name = name
	doc.Short = short
	doc.Long = strings.Join(long, "\n")
	doc.Examples = strings.Join(examples, "\n")

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return doc
}

type doc struct {
	Name     string
	Short    string
	Long     string
	Examples string
}

func (d doc) String() string {
	var parts []string

	if d.Short != "" {
		parts = append(parts,
			fmt.Sprintf("var %sShort=`%s`", d.Name, d.Short))
	}
	if d.Long != "" {
		parts = append(parts,
			fmt.Sprintf("var %sLong=`%s`", d.Name, d.Long))
	}
	if d.Examples != "" {
		parts = append(parts,
			fmt.Sprintf("var %sExamples=`%s`", d.Name, d.Examples))
	}

	return strings.Join(parts, "\n") + "\n"
}
