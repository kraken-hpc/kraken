package copy

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
)

// ErrorSkip can be returned by PreHook to skip a file
// when copying.
type ErrorSkip struct {
	// File is what is being skipped.
	File string
}

func (e *ErrorSkip) Error() string { return "skipped: " + e.File }

func NewErrorSkip(file string) *ErrorSkip { return &ErrorSkip{File: file} }

// Options provides configuration for copying.
type Options struct {
	// Recurse into subdirectories, copying their
	// contents.
	Recursive bool

	// Clobber (overwrite) file or directory if it
	// already exists.
	Clobber bool

	// If FollowSymlinks is set, copy the file pointed to by
	// the symlink instead of the symlink itself.
	FollowSymlinks bool

	// Be verbose, showing each file being copied.
	Verbose bool

	// PreHook is called, if specified, on each file before
	// it is copied.
	//
	// If PreHook returns ErrorSkip, the current file
	// is skipped and Copy returns nil.
	//
	// If PreHook returns another non-nil error, the
	// current file is skipped and Copy returns the error.
	PreHook func(src, dst string, srcInfo os.FileInfo) error

	// PostHook is called, if specified, on the current
	// file after it is copied.
	PostHook func(src, dst string)
}

// Copy copies a regular file, symlink, or directory
// located at src to the destination dst, using the
// options configured.
func (opt Options) Copy(src, dst string) error {
	var srcInfo os.FileInfo
	var err error

	// Make sure source file exists
	srcInfo, err = opt.Stat(src)
	if err != nil {
		return err
	}

	// Call pre-copy hook if specified
	if opt.PreHook != nil {
		err := opt.PreHook(src, dst, srcInfo)
		if _, ok := err.(*ErrorSkip); ok {
			return nil
		} else if err != nil {
			return err
		}
	}

	// Only overwrite dst if Clobber is true
	if err := opt.copyFile(src, dst, srcInfo); err != nil {
		return err
	}

	// Call post-copy hook if specified
	if opt.PostHook != nil {
		opt.PostHook(src, dst)
	}

	return nil
}

// copyFile determines which copy helper function to call
// based on the type of file src is.
func (opt Options) copyFile(src, dst string, srcInfo os.FileInfo) error {
	if srcInfo == nil {
		return fmt.Errorf("srcInfo (os.FileInfo) cannot be nil")
	}
	srcMode := srcInfo.Mode()

	switch {
	// Directory
	case srcMode.IsDir():
		return opt.copyTree(src, dst, srcInfo)

	// Regular file
	case srcMode.IsRegular():
		return opt.copyRegularFile(src, dst, srcInfo)

	// Symlink
	case srcMode&os.ModeSymlink == os.ModeSymlink:
		return opt.handleSymlink(src, dst)

	// Anything else
	default:
		return &os.PathError{
			Op:   "copy",
			Path: src,
			Err:  fmt.Errorf("unsupported file mode %s", srcMode),
		}
	}
}

// copyRegularFile copies a regular file located at src to the
// destination dst.
func (opt Options) copyRegularFile(src, dst string, srcInfo os.FileInfo) error {
	if opt.Verbose {
		log.Printf("[F] %q -> %q\n", src, dst)
	}

	// Open source file for reading
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Open destination file for creation/writing
	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// handleSymlink copies a symlink src to dst by reading it and
// creating a new symlink.
func (opt Options) handleSymlink(src, dst string) error {
	// Get symlink's destination
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}

	if opt.Verbose {
		log.Printf("[L] %q (%s) -> %q\n", src, target, dst)
	}

	if opt.FollowSymlinks {
		// Recursively follow symlink
		if _, err := os.Lstat(target); err != nil {
			return err
		}
		return opt.Copy(target, dst)
	} else {
		// "Copy" symlink itself
		if err := os.Symlink(target, dst); err != nil {
			if opt.Clobber {
				if err := os.RemoveAll(dst); err != nil {
					return err
				}
				return os.Symlink(target, dst)
			} else {
				return err
			}
		}
		return nil
	}
}

// copyTree copies a directory at src and its contents, recursively,
// to the destination dst.
func (opt Options) copyTree(src, dst string, info os.FileInfo) error {
	if opt.Verbose {
		log.Printf("[D] %q -> %q\n", src, dst)
	}

	// TODO: Add preservation of mode
	if info == nil {
		return fmt.Errorf("info (os.FileInfo) cannot be nil")
	} else if !info.Mode().IsDir() {
		return fmt.Errorf("%q is not a directory", src)
	}

	// Make destination dir
	err := os.Mkdir(dst, info.Mode())
	if err != nil {
		if opt.Clobber {
			if err = os.RemoveAll(dst); err != nil {
				return err
			}
			if err = os.Mkdir(dst, info.Mode()); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if opt.Recursive {
		contents, err := ioutil.ReadDir(src)
		if err != nil {
			return err
		}
		for _, item := range contents {
			srcItem, dstItem := path.Join(src, item.Name()), path.Join(dst, item.Name())
			if _, err := opt.Stat(srcItem); err != nil {
				return err
			}
			if err := opt.Copy(srcItem, dstItem); err != nil {
				return err
			}
		}
	}
	return nil
}

// Stat returns a file's information. If path is a symlink,
// then Stat does or does not follow it based on the value
// of FollowSymLinks.
func (opt Options) Stat(path string) (os.FileInfo, error) {
	if opt.FollowSymlinks {
		return os.Stat(path)
	} else {
		return os.Lstat(path)
	}
}

// GetDefaultOptions returns an Options struct populated
// with the default options.
func GetDefaultOptions() Options {
	return Options{
		Recursive:      false, // Don't copy dir contents
		Clobber:        true,  // cp overwrites by default
		FollowSymlinks: true,  // cp by default copies targets
		Verbose:        false, // Don't print filenames
		PreHook:        nil,
		PostHook:       nil,
	}
}

// Copy uses the default options to copy
// src to dst.
func Copy(src, dst string) error {
	var defaultOpts = GetDefaultOptions()
	return defaultOpts.Copy(src, dst)
}
