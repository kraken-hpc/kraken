/* buildutil.go: provides useful methods used in building Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *         Devon Bautista <devontbautista@gmail.com>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package lib

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SimpleSearchAndReplace searches all lines in a file and replaces all
// instances of the search string with the replace string.
// This currently only works with regular text files.
// Use DeepSearchandReplace if directories must be considered.
func SimpleSearchAndReplace(filename, srchStr, replStr string) (e error) {
	// Try to open file
	input, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalf("error opening file %s for search and replace", filename)
	}

	// Put lines of file into array
	lines := strings.Split(string(input), "\n")

	// Iterate through lines to search and replace
	for i, line := range lines {
		if strings.Contains(line, srchStr) {
			// Replace srchStr with replStr in line[i]
			lines[i] = strings.ReplaceAll(string(lines[i]), srchStr, replStr)
		}
	}

	// Merge array into one string
	output := strings.Join(lines, "\n")

	// Attempt to write new contents to file
	e = ioutil.WriteFile(filename, []byte(output), 0644)
	if e != nil {
		log.Fatal("error writing file during search and replace")
	}
	return
}

// SimpleRegexReplace applies a regular expression to the passed file and replaces
// matches with a string which supports capturing groups, etc.
func SimpleRegexReplace(filename, src, repl string) (e error) {
	// Get file perms
	var fInfo os.FileInfo
	if fInfo, e = os.Stat(filename); e != nil {
		e = fmt.Errorf("could not stat %s: %v", filename, e)
		return
	}

	// Open file
	var fileContents []byte
	if fileContents, e = ioutil.ReadFile(filename); e != nil {
		e = fmt.Errorf("could not read %s: %v", filename, e)
		return
	}

	// Replace src with repl
	var newContents []byte
	re := regexp.MustCompile(src)
	newContents = re.ReplaceAll(fileContents, []byte(repl))

	// Replace file
	if e = ioutil.WriteFile(filename, newContents, fInfo.Mode()); e != nil {
		e = fmt.Errorf("could not write %s: %v", filename, e)
	}

	return
}

// DeepSearchAndReplace performs SimpleSearchAndReplace on the passed file if
// it is a regular file. If the file is a directory, it recursively traverses it,
// performing SimpleSearchAndReplace on all regular files.
func DeepSearchAndReplace(filename, srchStr, replStr string) (e error) {
	// Get info of filename
	var info os.FileInfo
	info, e = os.Lstat(filename)
	if e != nil {
		return
	}

	// Is this a directory?
	if info.IsDir() {
		// If so, read each child and recurse until regular file found.
		var contents []os.FileInfo
		contents, e = ioutil.ReadDir(filename)
		for _, content := range contents {
			e = DeepSearchAndReplace(filepath.Join(filename, content.Name()), srchStr, replStr)
			if e != nil {
				return
			}
		}
	} else {
		// If not, perform a simple search and replace on the file
		e = SimpleSearchAndReplace(filename, srchStr, replStr)
	}

	return
}

// DeepRegexReplace performs SimpleRegexReplace on the passed file if it is
// a regular file. If the file is a directory, it recursively traverses it,
// performing SimpleRegexReplace on all of its regular files.
func DeepRegexReplace(filename, src, repl string) (e error) {
	// What type of file is this?
	var info os.FileInfo
	if info, e = os.Lstat(filename); e != nil {
		e = fmt.Errorf("could not stat %s: %v", filename, e)
		return
	}

	// If file is a directory, recurse until regular file is found
	if info.IsDir() {
		var contents []os.FileInfo
		if contents, e = ioutil.ReadDir(filename); e != nil {
			e = fmt.Errorf("could not read %s: %v", filename, e)
			return
		}
		for _, file := range contents {
			filePath := filepath.Join(filename, file.Name())
			if e = DeepRegexReplace(filePath, src, repl); e != nil {
				e = fmt.Errorf("could not deep regex replacement in %s: %v", filePath, e)
				return
			}
		}
	} else {
		if e = SimpleRegexReplace(filename, src, repl); e != nil {
			e = fmt.Errorf("could not perform simple regex replacement on %s: %v", filename, e)
		}
	}

	return
}
