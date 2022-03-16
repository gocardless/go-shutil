package shutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type SameFileError struct {
	Src string
	Dst string
}

func (e SameFileError) Error() string {
	return fmt.Sprintf("%s and %s are the same file", e.Src, e.Dst)
}

type SpecialFileError struct {
	File     string
	FileInfo os.FileInfo
}

func (e SpecialFileError) Error() string {
	return fmt.Sprintf("`%s` is a named pipe", e.File)
}

type NotADirectoryError struct {
	Src string
}

func (e NotADirectoryError) Error() string {
	return fmt.Sprintf("`%s` is not a directory", e.Src)
}

type AlreadyExistsError struct {
	Dst string
}

func (e AlreadyExistsError) Error() string {
	return fmt.Sprintf("`%s` already exists", e.Dst)
}

type MoveOntoSelfError struct {
	Src string
	Dst string
}

func (e MoveOntoSelfError) Error() string {
	return fmt.Sprintf("Cannot move a directory `%s` into itself `%s` ", e.Src, e.Dst)
}

func samefile(src string, dst string) bool {
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)
	return os.SameFile(srcInfo, dstInfo)
}

func specialfile(fi os.FileInfo) bool {
	return (fi.Mode() & os.ModeNamedPipe) == os.ModeNamedPipe
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func IsSymlink(fi os.FileInfo) bool {
	return (fi.Mode() & os.ModeSymlink) == os.ModeSymlink
}

// Copy data from src to dst
//
// If followSymlinks is not set and src is a symbolic link, a
// new symlink will be created instead of copying the file it points
// to.
func CopyFile(src, dst string, followSymlinks bool) error {
	if samefile(src, dst) {
		return &SameFileError{src, dst}
	}

	// Make sure src exists and neither are special files
	srcStat, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if specialfile(srcStat) {
		return &SpecialFileError{src, srcStat}
	}

	dstStat, err := os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		if specialfile(dstStat) {
			return &SpecialFileError{dst, dstStat}
		}
	}

	// If we don't follow symlinks and it's a symlink, just link it and be done
	if !followSymlinks && IsSymlink(srcStat) {
		return os.Symlink(src, dst)
	}

	// If we are a symlink, follow it
	if IsSymlink(srcStat) {
		src, err = os.Readlink(src)
		if err != nil {
			return err
		}
		srcStat, err = os.Stat(src)
		if err != nil {
			return err
		}
	}

	// Do the actual copy
	fsrc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fsrc.Close()

	fdst, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fdst.Close()

	size, err := io.Copy(fdst, fsrc)
	if err != nil {
		return err
	}

	if size != srcStat.Size() {
		return fmt.Errorf("%s: %d/%d copied", src, size, srcStat.Size())
	}

	return nil
}

// Copy mode bits from src to dst.
//
// If followSymlinks is false, symlinks aren't followed if and only
// if both `src` and `dst` are symlinks. If `lchmod` isn't available
// and both are symlinks this does nothing. (I don't think lchmod is
// available in Go)
func CopyMode(src, dst string, followSymlinks bool) error {
	srcStat, err := os.Lstat(src)
	if err != nil {
		return err
	}

	dstStat, err := os.Lstat(dst)
	if err != nil {
		return err
	}

	// They are both symlinks and we can't change mode on symlinks.
	if !followSymlinks && IsSymlink(srcStat) && IsSymlink(dstStat) {
		return nil
	}

	// Atleast one is not a symlink, get the actual file stats
	srcStat, _ = os.Stat(src)
	err = os.Chmod(dst, srcStat.Mode())
	return err
}

// Copy data and mode bits ("cp src dst"). Return the file's destination.
//
// The destination may be a directory.
//
// If followSymlinks is false, symlinks won't be followed. This
// resembles GNU's "cp -P src dst".
//
// If source and destination are the same file, a SameFileError will be
// rased.
func Copy(src, dst string, followSymlinks bool) (string, error) {
	dstInfo, err := os.Stat(dst)

	if err == nil && dstInfo.Mode().IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	if err != nil && !os.IsNotExist(err) {
		return dst, err
	}

	err = CopyFile(src, dst, followSymlinks)
	if err != nil {
		return dst, err
	}

	err = CopyMode(src, dst, followSymlinks)
	if err != nil {
		return dst, err
	}

	return dst, nil
}

type CopyFunc func(string, string, bool) (string, error)
type IgnoreFunc func(string, []os.FileInfo) []string

type CopyTreeOptions struct {
	Symlinks               bool
	IgnoreDanglingSymlinks bool
	CopyFunction           CopyFunc
	Ignore                 IgnoreFunc
}

// Recursively copy a directory tree.
//
// The destination directory must not already exist.
//
// If the optional Symlinks flag is true, symbolic links in the
// source tree result in symbolic links in the destination tree; if
// it is false, the contents of the files pointed to by symbolic
// links are copied. If the file pointed by the symlink doesn't
// exist, an error will be returned.
//
// You can set the optional IgnoreDanglingSymlinks flag to true if you
// want to silence this error. Notice that this has no effect on
// platforms that don't support os.Symlink.
//
// The optional ignore argument is a callable. If given, it
// is called with the `src` parameter, which is the directory
// being visited by CopyTree(), and `names` which is the list of
// `src` contents, as returned by ioutil.ReadDir():
//
//   callable(src, entries) -> ignoredNames
//
// Since CopyTree() is called recursively, the callable will be
// called once for each directory that is copied. It returns a
// list of names relative to the `src` directory that should
// not be copied.
//
// The optional copyFunction argument is a callable that will be used
// to copy each file. It will be called with the source path and the
// destination path as arguments. By default, Copy() is used, but any
// function that supports the same signature (like Copy2() when it
// exists) can be used.
func CopyTree(src, dst string, options *CopyTreeOptions) error {
	if options == nil {
		options = &CopyTreeOptions{
			Symlinks:               false,
			Ignore:                 nil,
			CopyFunction:           Copy,
			IgnoreDanglingSymlinks: false}
	}

	srcFileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcFileInfo.IsDir() {
		return &NotADirectoryError{src}
	}

	_, err = os.Open(dst)
	if !os.IsNotExist(err) {
		return &AlreadyExistsError{dst}
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, srcFileInfo.Mode())
	if err != nil {
		return err
	}

	ignoredNames := []string{}
	if options.Ignore != nil {
		ignoredNames = options.Ignore(src, entries)
	}

	for _, entry := range entries {
		if stringInSlice(entry.Name(), ignoredNames) {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		entryFileInfo, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		// Deal with symlinks
		if IsSymlink(entryFileInfo) {
			linkTo, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if options.Symlinks {
				os.Symlink(linkTo, dstPath)
				//CopyStat(srcPath, dstPath, false)
			} else {
				// ignore dangling symlink if flag is on
				_, err = os.Stat(linkTo)
				if os.IsNotExist(err) && options.IgnoreDanglingSymlinks {
					continue
				}
				_, err = options.CopyFunction(srcPath, dstPath, false)
				if err != nil {
					return err
				}
			}
		} else if entryFileInfo.IsDir() {
			err = CopyTree(srcPath, dstPath, options)
			if err != nil {
				return err
			}
		} else {
			_, err = options.CopyFunction(srcPath, dstPath, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Determines if a file represented
// by `path` is a directory or not
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}

type MoveOptions struct {
	CopyFunction CopyFunc
}

// Recursively move a file or directory to another location. this is similar to
// the unix "mv" command. Return the file or directory's destination.
//
// If the destination is a directory or a symlink to a directory, the source is
// moved inside the directory. The destination path must not exist.
//
// If the destination already exists but is not a directory, it may be overwritten
// depending on os.Rename() semantics.
//
// If the destination is in our current file system, then rename() is used. Otherwise,
// src is copied to the destination and then removed. Symlinks are recreated under the new
// name if os.rename() fails because of cross filesystem renames.
//
// The optional `copy_function` argument is a callable the will be used to copy the source
// or it will be delegated to `copytree`. By default copy2() is used, but any function
// that supports the same signature (like copy()) can be used.
//

func Move(src, dst string, options *MoveOptions) (string, error) {
	if options == nil {
		options = &MoveOptions{
			CopyFunction: Copy,
		}
	}
	real_dst := dst

	// dst might not exist so ignore any errors
	// (matching Pythons os.path.isdir())
	isDirDst, _ := isDirectory(dst)

	if isDirDst {
		if samefile(src, dst) {
			// We might be on a case insentive file system,
			// perform the rename anyway
			return dst, os.Rename(src, dst)
		}
		real_dst = path.Join(dst, path.Base(src))
		if _, err := os.Stat(real_dst); err == nil {
			return "", &AlreadyExistsError{dst}
		}
	}
	// If a rename works, do that
	if err := os.Rename(src, real_dst); err == nil {
		return real_dst, nil
	}

	srcStat, err := os.Lstat(src)
	if err != nil {
		return "", err
	}

	// If the source is a symlink then handle that
	if IsSymlink(srcStat) {
		linkto, err := os.Readlink(src)
		if err != nil {
			return "", err
		}
		err = os.Symlink(linkto, real_dst)
		if err != nil {
			return "", err
		}
		err = os.Remove(src)
		if err != nil {
			return "", err
		}
		return real_dst, nil
	}

	isSrcDir, _ := isDirectory(src)

	if isSrcDir {
		insrc, err := destinsrc(src, dst)
		if err != nil {
			return "", err
		}
		if insrc {
			return "", &MoveOntoSelfError{src, dst}
		}
		// Skip the immutability checks for now
		// These are hard in Golang
		CopyTree(src, real_dst, &CopyTreeOptions{
			Symlinks:               true,
			IgnoreDanglingSymlinks: false,
			Ignore:                 nil,
			CopyFunction:           Copy,
		})
		os.RemoveAll(src)
	} else {
		_, err = options.CopyFunction(src, real_dst, true)
		if err != nil {
			return "", err
		}
		err = os.Remove(src)
		if err != nil {
			return "", err
		}
	}
	return real_dst, nil

}

func destinsrc(src, dst string) (bool, error) {
	var err error
	sep := string(os.PathSeparator)

	src, err = filepath.Abs(src)
	if err != nil {
		return false, err
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return false, err
	}
	if !strings.HasSuffix(src, sep) {
		src += sep
	}
	if !strings.HasSuffix(src, sep) {
		dst += sep
	}
	return strings.HasPrefix(dst, src), nil
}
