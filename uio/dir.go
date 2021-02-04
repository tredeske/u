//+build linux

package uio

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
)

//
// compute the amount of space used by the files rooted at dir
//
func DiskUsage(dir string) (used int64, err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return used, err
	}
	for _, f := range files {
		if f.IsDir() {
			subUsed, err := DiskUsage(path.Join(dir, f.Name()))
			if err != nil {
				return used, err
			}
			used += subUsed
		} else if f.Mode().IsRegular() {
			used += f.Size()
		}
	}
	return used, nil
}

//
// copy src dir to dst dir
//
func DirCopy(src, dst string) (err error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return
	} else if !srcInfo.IsDir() {
		err = fmt.Errorf("%s is not a dir", src)
		return
	}

	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return
	}

	files, err := ioutil.ReadDir(src)
	for _, file := range files {

		srcPath := path.Join(src, file.Name())
		dstPath := path.Join(dst, file.Name())

		if file.IsDir() {
			err = DirCopy(srcPath, dstPath)
		} else {
			err = FileCopy(srcPath, dstPath)
		}
		if err != nil {
			break
		}
	}
	return
}

//
// Get the names of files contained by dir.  If max is specified, only
// return up to max names.
//
func DirFilenames(dir string, max ...int) (files []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	n := -1
	if 0 != len(max) {
		n = max[0]
	}
	files, err = f.Readdirnames(n)
	f.Close()
	return
}

func DirList(dir string) (rv string, err error) {
	files, err := FilesByModTime(dir)
	if err != nil {
		return
	}
	var bb bytes.Buffer
	bb.WriteString(dir)
	bb.WriteRune('\n')
	if 0 == len(files) {
		bb.WriteString("[no entries]\n")
	} else {
		for _, fi := range files {
			fmt.Fprintf(&bb, "%s %6d %s %s\n",
				fi.Mode(), fi.Size(), fi.ModTime(), fi.Name())
		}
	}
	rv = bb.String()
	return
}

//
// Is the dir empty?
//
func DirEmpty(dir string) bool {
	files, _ := DirFilenames(dir, 1)
	return 1 != len(files)
}

//
// is it a directory?
//
func FileIsDir(file string) bool {
	fi, err := os.Stat(file)
	return nil == err && fi.IsDir()
}

type file_by_mtime []os.FileInfo

func (f file_by_mtime) Len() int      { return len(f) }
func (f file_by_mtime) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f file_by_mtime) Less(i, j int) bool {
	return f[i].ModTime().Before(f[j].ModTime())
}

// Sorted by mtime (oldest to youngest)
func SortByModTime(files []os.FileInfo) {
	if 1 < len(files) {
		sort.Sort(file_by_mtime(files))
	}
}

//
// Get listing of dir, sorted by mtime (oldest to youngest)
//
func FilesByModTime(dir string) (files []os.FileInfo, err error) {
	dirF, err := os.Open(dir)
	if err != nil {
		return
	}
	defer dirF.Close()
	files, err = dirF.Readdir(0)
	if err != nil {
		return nil, err
	}
	SortByModTime(files)
	return
}
