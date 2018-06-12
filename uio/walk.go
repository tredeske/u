package uio

import (
	"os"
	"path/filepath"
)

//
// grabbed from stdlib (filepath), and tweaked so that if WalkFn does not
// return an error, we proceed anyway.  this allows for WalkFn to fix
// permissions, etc.
//
// walk recursively descends path, calling walkFn.
//
func walk(path string, info os.FileInfo, walkFn filepath.WalkFunc) error {

	if !info.IsDir() {
		return walkFn(path, info, nil)
	}

	names, err := ReadDirNames(path)
	errWalk := walkFn(path, info, err)

	if err != nil && nil == errWalk { // try again
		names, err = ReadDirNames(path)
		errWalk = err
	}

	// If err != nil, walk can't walk into this directory.
	// errWalk != nil means walkFn want walk to skip this directory or stop walking.
	// Therefore, if one of err and err1 isn't nil, walk will return.
	if err != nil || errWalk != nil {

		// The caller's behavior is controlled by the return value, which is decided
		// by walkFn. walkFn may ignore err and return nil.
		// If walkFn returns SkipDir, it will be handled by the caller.
		// So walk should return whatever walkFn returns.
		return errWalk
	}

	for _, name := range names {

		filename := filepath.Join(path, name)
		info, err = os.Lstat(filename)
		if err != nil {
			errWalk = walkFn(filename, info, err)
			if nil == errWalk { // try again
				info, err = os.Lstat(filename)
			} else if errWalk == filepath.SkipDir {
				continue
			}
			if err != nil {
				return err
			}
		}
		err = walk(filename, info, walkFn)
		if err != nil {
			if !info.IsDir() || err != filepath.SkipDir {
				return err
			}
		}
	}
	return nil
}

//
// ReadDirNames reads the directory named by dirname and returns
// a list of directory entries.
//
func ReadDirNames(dirname string) (names []string, err error) {

	f, err := os.Open(dirname)
	if err != nil {
		return
	}

	names, err = f.Readdirnames(-1)
	f.Close()
	if err != nil {
		names = nil
	}
	return
}

//
// grabbed from stdlib (filepath), and tweaked so that if WalkFn does not
// return an error, we proceed anyway.  this allows for WalkFn to fix
// permissions, etc.
//
// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn.  Walk does not follow symbolic links.
//
func Walk(root string, walkFn filepath.WalkFunc) (err error) {

	info, err := os.Lstat(root)
	if err != nil {
		err = walkFn(root, nil, err)
		if nil == err { // give it another go
			info, err = os.Lstat(root)
		}
	}
	if nil == err {
		err = walk(root, info, walkFn)
	}
	if err == filepath.SkipDir {
		return nil
	}
	return err
}
