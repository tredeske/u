package usftp

import (
	"io/fs"
	"path"
)

// implement fs interfaces
type FS interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
}

// implement fs.FS, fs.ReadDirFS, fs.StatFS
type fsClient_ struct {
	client *Client
}

// implement fs.FS
func (fsc *fsClient_) Open(name string) (fs.File, error) {
	f, err := fsc.client.OpenRead(name)
	if err != nil {
		return nil, err
	}
	return &fsFile_{f: f}, nil
}

// implement fs.StatFS
func (fsc *fsClient_) Stat(name string) (fs.FileInfo, error) {
	s, err := fsc.client.Stat(name)
	if err != nil {
		return nil, err
	}
	return FileInfoFromStat(s, path.Base(name)), nil
}

// implement fs.ReadDirFS
func (fsc *fsClient_) ReadDir(dirN string) ([]fs.DirEntry, error) {
	files, err := fsc.client.ReadDirLimit(dirN, 0, nil)
	if err != nil || 0 == len(files) {
		return nil, err
	}

	rv := make([]fs.DirEntry, len(files))
	for i, file := range files {
		rv[i] = &fsDirEntry_{
			info: FileInfoFromStat(&file.attrs, path.Base(file.Name())),
		}
	}
	return rv, nil
}

// implement fs.DirEntry
type fsDirEntry_ struct {
	info fs.FileInfo
}

func (de *fsDirEntry_) Name() string               { return de.info.Name() }
func (de *fsDirEntry_) IsDir() bool                { return de.info.IsDir() }
func (de *fsDirEntry_) Type() fs.FileMode          { return de.info.Mode().Type() }
func (de *fsDirEntry_) Info() (fs.FileInfo, error) { return de.info, nil }

// implement fs.File
type fsFile_ struct {
	f *File
}

// implement fs.File
func (fsf *fsFile_) Stat() (fs.FileInfo, error) {
	s, err := fsf.f.Stat()
	if err != nil {
		return nil, err
	}
	return FileInfoFromStat(s, path.Base(fsf.f.Name())), nil
}

// implement fs.File
func (fsf *fsFile_) Read(b []byte) (int, error) {
	return fsf.f.Read(b)
}

// implement fs.File
func (fsf *fsFile_) Close() error {
	return fsf.f.Close()
}
