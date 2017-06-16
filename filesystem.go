package ciel

import (
	"encoding/base64"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"syscall"
)

type FileSystem struct {
	lock sync.RWMutex

	WorkDir    string `role:"work"  dir:"99-workdir"`
	UpperDir   string `role:"upper" dir:"99-upperdir"`
	Cache      string `role:"lower" dir:"50-cache"`
	Buildkit   string `role:"lower" dir:"10-buildkit"`
	StubConfig string `role:"lower" dir:"01-stub-config"`
	Stub       string `role:"lower" dir:"00-stub"`

	base    string
	Target  string
	mounted bool
}

const _SYSTEMDPATH = "/usr/lib/systemd/systemd"

func (fs *FileSystem) isBootable() bool {
	fs.lock.RLock()
	defer fs.lock.RUnlock()

	if !fs.mounted {
		return false
	}
	if _, err := os.Stat(fs.Target + _SYSTEMDPATH); os.IsNotExist(err) {
		return false
	}
	return true
}

func (fs *FileSystem) isMounted() bool {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	return fs.mounted
}

func (fs *FileSystem) setBaseDir(path string) {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.mounted {
		panic("setBaseDir when file system has been mounted")
	}

	fs.base = path
	t := reflect.TypeOf(*fs)
	v := reflect.ValueOf(fs).Elem()
	n := t.NumField()
	for i := 0; i < n; i++ {
		role := t.Field(i).Tag.Get("role")
		dir := t.Field(i).Tag.Get("dir")
		if dir != "" {
			fulldir := fs.base + "/" + dir
			v.Field(i).SetString(fulldir)
			if role != "work" {
				os.Mkdir(fulldir, 0775)
			}
		}
	}
}

func (fs *FileSystem) mount() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if fs.mounted {
		return nil
	}

	lowerdirs := []string{}
	t := reflect.TypeOf(*fs)
	v := reflect.ValueOf(fs).Elem()
	n := t.NumField()
	for i := 0; i < n; i++ {
		role := t.Field(i).Tag.Get("role")
		if role == "lower" {
			lowerdirs = append(lowerdirs, v.Field(i).String())
		}
	}
	fs.Target = "/tmp/ciel." + randomFilename()
	os.Mkdir(fs.Target, 0775)
	os.Mkdir(fs.WorkDir, 0775)
	reterr := mount(fs.Target, fs.UpperDir, fs.WorkDir, lowerdirs...)
	if reterr == nil {
		fs.mounted = true
	}
	return reterr
}

func (fs *FileSystem) unmount() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if !fs.mounted {
		return nil
	}

	if err := unmount(fs.Target); err != nil {
		return err
	}
	defer func() {
		fs.mounted = false
	}()
	err1 := os.Remove(fs.Target)
	err2 := os.RemoveAll(fs.WorkDir)
	if err2 != nil {
		return err2
	}
	if err1 != nil {
		return err1
	}
	return nil
}

func randomFilename() string {
	const SIZE = 8
	rd := make([]byte, SIZE)
	if _, err := rand.Read(rd); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(rd)
}
func mount(path string, upperdir string, workdir string, lowerdirs ...string) error {
	return syscall.Mount("overlay", path, "overlay", 0,
		"lowerdir="+strings.Join(lowerdirs, ":")+",upperdir="+upperdir+",workdir="+workdir)
}
func unmount(path string) error {
	return syscall.Unmount(path, 0)
}
