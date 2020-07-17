// Package cmp provides file- and tree-comparison routines.
//
// It is a sub-package to copy's test suite.
package cmp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/hpc/kraken/lib/copy"
)

const (
	// Compare this many bytes of a file at a time
	chunkSize = 64000 // 64KB
)

// IsSameFile returns true if file1 and file2 are the
// same file as determined by os.SameFile.
// This will work with directories as well.
// What to do with symlinks is configured by opt.
func IsSameFile(opt copy.Options, file1, file2 string) (bool, error) {
	var fileInfo1, fileInfo2 os.FileInfo
	var err error

	fileInfo1, err = opt.Stat(file1)
	if err != nil {
		return false, err
	}
	fileInfo2, err = opt.Stat(file2)
	if err != nil {
		return false, err
	}

	return os.SameFile(fileInfo1, fileInfo2), err
}

// readDirNames returns an array of file names of the
// contents of the directory at path.
func readDirNames(path string) ([]string, error) {
	items, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var basenames []string
	for _, item := range items {
		basenames = append(basenames, item.Name())
	}
	return basenames, nil
}

// IsEqualSymlink checks if two symlinks have the same name and target.
func IsEqualSymlink(link1, link2 string, lInfo1, lInfo2 os.FileInfo) error {
	// Check targets
	targ1, err := os.Readlink(link1)
	if err != nil {
		return err
	}
	targ2, err := os.Readlink(link2)
	if err != nil {
		return err
	}
	if targ1 != targ2 {
		return fmt.Errorf("symlinks point to different locations:\n%q -> %q\n%q -> %q", link1, targ1, link2, targ2)
	}
	return nil
}

// areFilesOrEqualSymlinks returns nil if (1) the files are
// both symlinks AND they point to the same location or (2)
// neither file is a symlink. Otherwise, it returns an error.
func areFilesOrEqualSymlinks(opt copy.Options, fPath1, fPath2 string, fInfo1, fInfo2 os.FileInfo) (file1, file2 string, err error) {
	fIsSymLink1 := fInfo1.Mode()&os.ModeSymlink == os.ModeSymlink
	fIsSymLink2 := fInfo2.Mode()&os.ModeSymlink == os.ModeSymlink
	file1 = fPath1
	file2 = fPath2

	// Check if either or both files are symlinks
	if fIsSymLink1 || fIsSymLink2 {
		if opt.FollowSymlinks {
			// If we're following symlinks, find their targets in order
			// to compare the end results
			file1, err = filepath.EvalSymlinks(file1)
			if err != nil {
				return
			}
			file2, err = filepath.EvalSymlinks(file2)
			if err != nil {
				return
			}
		} else {
			if fIsSymLink1 && fIsSymLink2 {
				// If BOTH are symlinks and we're not following them,
				// compare their targets
				err = IsEqualSymlink(file1, file2, fInfo1, fInfo2)
			} else {
				// Only one of the files is a symlink and we're not
				// following them, they are not considered the same
				err = fmt.Errorf("one file is symlink, one is regular file: %q, %q", file1, file2)
			}
		}
	}
	return
}

// IsSameFile compares two files by byte contents.
// What to do with symlinks is configured by opt.
func IsEqualFile(opt copy.Options, file1, file2 string) error {
	// What kind of files are these?
	fInfo1, err := opt.Stat(file1)
	if err != nil {
		return err
	}
	fInfo2, err := opt.Stat(file2)
	if err != nil {
		return err
	}
	if file1, file2, err = areFilesOrEqualSymlinks(opt, file1, file2, fInfo1, fInfo2); err != nil {
		return err
	}

	// Open files
	f1, err := os.Open(file1)
	if err != nil {
		return err
	}
	defer f1.Close()
	f2, err := os.Open(file2)
	if err != nil {
		return err
	}
	defer f2.Close()

	// Compare chunks of files until different or EOF
	// in one or both
	for {
		// Create buffers in loop since Go is garbage-collected
		buf1 := make([]byte, chunkSize)
		_, err1 := f1.Read(buf1)

		buf2 := make([]byte, chunkSize)
		_, err2 := f2.Read(buf2)

		// Check for EOF
		if err1 != nil || err2 != nil {
			if errors.Is(err1, io.EOF) && errors.Is(err2, io.EOF) {
				// EOF for both files reached; files equal
				return nil
			} else if errors.Is(err1, io.EOF) || errors.Is(err2, io.EOF) {
				// Files different lengths; not equal
				return fmt.Errorf("files different lengths: %s, %s", file1, file2)
			} else {
				return fmt.Errorf("non-EOF errors thrown: %s, %s", err1, err2)
			}
		}

		// Compare buffers
		if !bytes.Equal(buf1, buf2) {
			return fmt.Errorf("%q and %q do not have equal content", file1, file2)
		}
	}
}

// IsEqualDir compares the contents of two directories.
// What to do with symlinks is configured by opt.
func IsEqualDir(opt copy.Options, src, dst string) error {
	// Get info for each dir
	srcInfo, err := opt.Stat(src)
	if err != nil {
		return err
	}
	dstInfo, err := opt.Stat(dst)
	if err != nil {
		return err
	}

	// Do they have the same mode?
	if srcMode, dstMode := srcInfo.Mode()&os.ModeType, dstInfo.Mode()&os.ModeType; srcMode != dstMode {
		return fmt.Errorf("mismatched mode: %q (%s) and %q (%s)", src, srcMode, dst, dstMode)
	}

	// Decide what to do based on file type (for recursion)
	if srcInfo.Mode().IsDir() {
		// Directory
		if opt.Recursive {
			// Compare directory contents' names
			srcNames, err := readDirNames(src)
			if err != nil {
				return err
			}
			dstNames, err := readDirNames(dst)
			if err != nil {
				return err
			}

			if !reflect.DeepEqual(srcNames, dstNames) {
				return fmt.Errorf("directory contents mismatch:\n%q: %v\n%q: %v", src, srcNames, dst, dstNames)
			}

			// Recursively compare directory contents
			for _, item := range srcNames {
				if err := IsEqualDir(opt, filepath.Join(src, item), filepath.Join(dst, item)); err != nil {
					return err
				}
			}
			return nil
		} else {
			if srcInfo.Mode() == dstInfo.Mode() {
				return nil
			}
			return fmt.Errorf("directory mode mismatch: %q (%v), %q (%v)", src, srcInfo.Mode(), dst, dstInfo.Mode())
		}
	} else if srcInfo.Mode().IsRegular() || srcInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		// Regular file or symlink
		return IsEqualFile(opt, src, dst)
	} else {
		return fmt.Errorf("unsupported file mode: %s", srcInfo.Mode())
	}
}
