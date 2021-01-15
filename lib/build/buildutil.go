/* buildutil.go: provides useful methods used in building Kraken
 *
 * Author: Devon Bautista <devontbautista@gmail.com>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package build

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// simpleSearchAndReplace searches all lines in a file and replaces all
// instances of srchStr with replStr.
// This is a helper function for SearchAndReplace only works with regular
// files.
func simpleSearchAndReplace(filename, srchStr, replStr string) (e error) {
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

// simpleRegexReplace applies a regular expression src to filename and replaces
// matches with repl, which supports formatting specified in the regexp package.
// This is a helper function for RegexReplace and only works on regular files.
func simpleRegexReplace(filename, src, repl string) (e error) {
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

// SearchAndReplace replaces all instances of srchStr in filename with replStr.
// If filename is a directory, it does this recursively for all files and
// directories contained in it.
func SearchAndReplace(filename, srchStr, replStr string) (e error) {
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
			e = SearchAndReplace(filepath.Join(filename, content.Name()), srchStr, replStr)
			if e != nil {
				return
			}
		}
	} else {
		// If not, perform a simple search and replace on the file
		e = simpleSearchAndReplace(filename, srchStr, replStr)
	}

	return
}

// RegexReplace applies the regular expression src to filename and replaces
// matches with repl. If filename is a directory, it does this recursively
// for all files and directories contained in it. The replace string repl
// supports formatting according to the regexp package.
func RegexReplace(filename, src, repl string) (e error) {
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
			if e = RegexReplace(filePath, src, repl); e != nil {
				e = fmt.Errorf("could not deep regex replacement in %s: %v", filePath, e)
				return
			}
		}
	} else {
		if e = simpleRegexReplace(filename, src, repl); e != nil {
			e = fmt.Errorf("could not perform simple regex replacement on %s: %v", filename, e)
		}
	}

	return
}
