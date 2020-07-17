package copy_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"regexp"
	"testing"

	"github.com/hpc/kraken/lib/copy"
	"github.com/hpc/kraken/lib/copy/cmp"
)

const (
	MaxFileSize = 1000 // In bytes
	MaxDirDepth = 5    // For generating test trees
	MaxNumFiles = 5    // Per directory
)

// randomFile creates a random file like os.TempFile, but also adds
// random content.
func randomFile(path, prefix string) (*os.File, error) {
	f, err := ioutil.TempFile(path, prefix)
	if err != nil {
		return nil, err
	}

	var bytes = []byte{}
	for i := 0; i < rand.Intn(MaxFileSize); i++ {
		bytes = append(bytes, byte(i))
	}
	f.Write(bytes)

	return f, nil
}

// randomTree creates a file tree (directory) of a certain depth
// containing a number of files (within the maximum), each
// containing random content.
func randomTree(root, prefix string, depth int) (string, error) {
	// Create first subdirectory
	rootPath, err := ioutil.TempDir(root, prefix+"0_")
	if err != nil {
		return "", err
	}

	// Create the rest of the subdirectories
	currDir := path.Join(root, rootPath)
	for currDepth := 1; currDepth < depth; currDepth++ {
		// Create next directory
		newDir, err := ioutil.TempDir(currDir, fmt.Sprintf(prefix+"%d_", currDepth))
		if err != nil {
			return "", err
		}

		// Create files with random content in each level
		for i := 0; i < MaxNumFiles; i++ {
			_, err := randomFile(currDir, fmt.Sprintf(prefix+"%d_", currDepth))
			if err != nil {
				return "", err
			}
		}

		currDir = newDir
	}
	return rootPath, nil
}

// copyFileAndTest is a wrapper that performs Copy() with the passed
// options and compares the result with the source file using
// cmp.IsEqualFile().
func copyFileAndTest(t *testing.T, opt copy.Options, src, dst string) {
	if err := opt.Copy(src, dst); err != nil {
		t.Fatalf("Copy(%q -> %q) = %v, want %v", src, dst, err, nil)
	}
	if err := cmp.IsEqualFile(opt, src, dst); err != nil {
		t.Fatalf("Expected %q and %q to be the same, got %v", src, dst, err)
	}
}

// copyDirAndTest is a wrapper that performs Copy() with the passed
// options and compares the result with the source directory using
// cmp.IsEqualTree().
func copyDirAndTest(t *testing.T, opt copy.Options, src, dst string) {
	if err := opt.Copy(src, dst); err != nil {
		t.Fatalf("Copy(%q -> %q) = %v, want %v", src, dst, err, nil)
	}
	if err := cmp.IsEqualDir(opt, src, dst); err != nil {
		t.Fatalf("Expected %q and %q to be the same, got %v", src, dst, err)
	}
}

// TestCopyFile tests copying a regular file.
func TestCopyFile(t *testing.T) {
	f, err := randomFile("", "testfile-")
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	fName := f.Name()
	defer os.RemoveAll(fName)

	fNameCopy := fName + "-copy"
	defaultOpts := copy.GetDefaultOptions()
	copyFileAndTest(t, defaultOpts, fName, fNameCopy)
	defer os.RemoveAll(fNameCopy)
}

// TestCopyDir tests copying a directory with
// and without Recursive option set.
func TestCopyDir(t *testing.T) {
	// Create source directory
	dPath, err := randomTree("", "test_dir-", MaxDirDepth)
	if err != nil {
		t.Fatalf("Unable to generate random directory tree: %v", err)
	}
	defer os.RemoveAll(dPath)

	// Test shallow copy
	NoRecursive := copy.Options{
		Recursive: false,
	}
	dPathCopyShallow := dPath + "-copy-shallow"
	copyDirAndTest(t, NoRecursive, dPath, dPathCopyShallow)
	defer os.RemoveAll(dPathCopyShallow)

	// Test deep copy
	Recursive := copy.Options{
		Recursive: true,
	}
	dPathCopyDeep := dPath + "-copy-deep"
	copyDirAndTest(t, Recursive, dPath, dPathCopyDeep)
	defer os.RemoveAll(dPathCopyDeep)
}

// TestCopySymlink tests copying a symlink, both the symlink itself
// and the file it points to based on the FollowSymlinks option.
func TestCopySymlink(t *testing.T) {
	// Create configs
	noFollowSymlink := copy.Options{
		FollowSymlinks: false,
	}
	followSymlink := copy.Options{
		FollowSymlinks: true,
	}

	tmpPath := os.TempDir()

	// Create target file
	targ, err := ioutil.TempFile("", "testtarget-")
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	targPath := targ.Name()
	defer os.RemoveAll(targPath)

	// Create symlink to target
	symName := "testsym-" + path.Base(targPath)
	symPath := path.Join(tmpPath, symName)
	err = os.Symlink(targPath, symPath)
	if err != nil {
		t.Fatalf("Unable to create symlink: %v", err)
	}
	defer os.RemoveAll(symPath)

	// Test copying symlink itself
	symNameCopy := symName + "-copy"
	symPathCopy := path.Join(tmpPath, symNameCopy)
	copyFileAndTest(t, noFollowSymlink, symPath, symPathCopy)
	defer os.RemoveAll(symPathCopy)

	// Test copying symlink target
	targPathCopy := targPath + "-copy"
	copyFileAndTest(t, followSymlink, symPath, targPathCopy)
	defer os.RemoveAll(targPathCopy)
}

// TestClobber tests overwriting a file/directory/symlink.
func TestClobber(t *testing.T) {
	// Create config
	clobber := copy.Options{
		Clobber: true,
	}

	tmpPath := os.TempDir()

	// Regular file
	f, err := randomFile("", "testfile-")
	if err != nil {
		t.Fatalf("Unable to create random file: %v", err)
	}
	fPath := f.Name()
	defer os.RemoveAll(fPath)
	fPathCopy := fPath + "-copy"
	copyFileAndTest(t, clobber, fPath, fPathCopy)
	defer os.RemoveAll(fPathCopy)
	copyFileAndTest(t, clobber, fPath, fPathCopy)

	// Directory
	d, err := randomTree("", "test_clobber-", MaxDirDepth)
	if err != nil {
		t.Fatalf("Unable to generate random directory tree: %v", err)
	}
	defer os.RemoveAll(d)
	dCopy := d + "-copy"

	clobber.Recursive = false
	copyDirAndTest(t, clobber, d, dCopy)
	defer os.RemoveAll(dCopy)
	copyDirAndTest(t, clobber, d, dCopy)

	clobber.Recursive = true
	copyDirAndTest(t, clobber, d, dCopy)
	copyDirAndTest(t, clobber, d, dCopy)

	// Symlink
	symName := "testsym-" + path.Base(fPath)
	symPath := path.Join(tmpPath, symName)
	err = os.Symlink(fPath, symPath)
	if err != nil {
		t.Fatalf("Unable to create symlink: %v", err)
	}
	defer os.RemoveAll(symPath)

	symPathCopy := symPath + "-copy"

	clobber.FollowSymlinks = false
	copyFileAndTest(t, clobber, symPath, symPathCopy)
	defer os.RemoveAll(symPathCopy)
	copyFileAndTest(t, clobber, symPath, symPathCopy)

	clobber.FollowSymlinks = true
	copyFileAndTest(t, clobber, symPath, fPathCopy)
	copyFileAndTest(t, clobber, symPath, fPathCopy)
}

// TestSkip tests skipping a file using a pre-copy
// and post-copy hook.
func TestSkip(t *testing.T) {
	fPrefix := "testfile-"
	fCopySuffix := "-copy"
	fpr := regexp.MustCompile(`.*` + fPrefix + `.*`)
	fsr := regexp.MustCompile(`.*` + fPrefix + `[a-zA-Z0-9]+` + fCopySuffix)

	// Define hooks
	skip := copy.GetDefaultOptions()
	skip.PreHook = func(src, dst string, srcInfo os.FileInfo) error {
		if fpr.MatchString(src) {
			return &copy.ErrorSkip{
				File: src,
			}
		}
		return nil
	}
	skip.PostHook = func(src, dst string) {
		// Check if file got skipped
		if fsr.MatchString(dst) {
			if _, err := os.Stat(dst); err == nil {
				t.Fatalf("file %q exists (%q should have been skipped)", dst, src)
				os.RemoveAll(dst)
			}
		}
	}

	// Create file
	f, err := randomFile("", fPrefix)
	if err != nil {
		t.Fatalf("Unable to create random file: %v", err)
	}
	fName := f.Name()
	defer os.RemoveAll(fName)

	fNameCopy := fName + "-copy"
	if err := skip.Copy(fName, fNameCopy); err != nil {
		t.Fatalf("Copy(%q -> %q) = %v, want %v", fName, fNameCopy, err, nil)
	}
}
