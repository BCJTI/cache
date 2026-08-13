// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FileCacheItem is basic unit of file cache adapter.
// it contains data and expire time.
type FileCacheItem struct {
	Data       interface{}
	Lastaccess time.Time
	Expired    time.Time
}

// FileCache Config
var (
	FileCachePath           = "cache"     // cache directory
	FileCacheFileSuffix     = ".bin"      // cache file suffix
	FileCacheDirectoryLevel = 2           // cache file deep level if auto generated cache files.
	FileCacheEmbedExpiry    time.Duration // cache expire time, default is no expire forever.
)

// foreverDuration is the expiry applied to items cached "forever".
const foreverDuration = (86400 * 365 * 100) * time.Second // ten years

// FileCache is cache adapter for file storage.
type FileCache struct {
	lock           sync.Mutex // serializes writes: Put, Delete and the counters' read-modify-write cycle
	CachePath      string
	FileSuffix     string
	DirectoryLevel int
	EmbedExpiry    int // in seconds; expiry for counter keys created by IncrBy/DecrBy (0 means forever)
}

// NewFileCache Create new file cache with no config.
// the level and expiry need set in method StartAndGC as config string.
func NewFileCache() Cache {
	//    return &FileCache{CachePath:FileCachePath, FileSuffix:FileCacheFileSuffix}
	return &FileCache{}
}

// StartAndGC will start and begin gc for file cache.
// the config need to be like {CachePath:"/cache","FileSuffix":".bin","DirectoryLevel":"2","EmbedExpiry":"0"}
func (fc *FileCache) StartAndGC(config string) error {

	cfg := make(map[string]string)
	err := json.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return err
	}
	if _, ok := cfg["CachePath"]; !ok {
		cfg["CachePath"] = FileCachePath
	}
	if _, ok := cfg["FileSuffix"]; !ok {
		cfg["FileSuffix"] = FileCacheFileSuffix
	}
	if v, ok := cfg["DirectoryLevel"]; !ok || v == "" {
		cfg["DirectoryLevel"] = strconv.Itoa(FileCacheDirectoryLevel)
	}
	if v, ok := cfg["EmbedExpiry"]; !ok || v == "" {
		cfg["EmbedExpiry"] = strconv.FormatInt(int64(FileCacheEmbedExpiry.Seconds()), 10)
	}
	fc.CachePath = cfg["CachePath"]
	fc.FileSuffix = cfg["FileSuffix"]
	if fc.DirectoryLevel, err = strconv.Atoi(cfg["DirectoryLevel"]); err != nil {
		return fmt.Errorf("invalid DirectoryLevel %q: %v", cfg["DirectoryLevel"], err)
	}
	if fc.EmbedExpiry, err = strconv.Atoi(cfg["EmbedExpiry"]); err != nil {
		return fmt.Errorf("invalid EmbedExpiry %q: %v", cfg["EmbedExpiry"], err)
	}
	if fc.EmbedExpiry < 0 || int64(fc.EmbedExpiry) > int64(foreverDuration/time.Second) {
		return fmt.Errorf("invalid EmbedExpiry %q: out of range (0 to %d seconds)", cfg["EmbedExpiry"], int64(foreverDuration/time.Second))
	}

	fc.Init()
	return nil
}

// Init will make new dir for file cache if not exist.
// it also sweeps temp files orphaned by writes that crashed between the
// temp-file creation and the rename in FilePutContents.
func (fc *FileCache) Init() {
	if ok, _ := exists(fc.CachePath); !ok { // todo : error handle
		_ = os.MkdirAll(fc.CachePath, os.ModePerm) // todo : error handle
	}
	fc.cleanOrphanTempFiles()
}

// cleanOrphanTempFiles removes stale "*.tmp*" leftovers from FilePutContents
// writes that never reached the rename (process crash). only files older
// than an hour are removed, so in-flight writes are never touched.
func (fc *FileCache) cleanOrphanTempFiles() {
	cutoff := time.Now().Add(-time.Hour)
	filepath.Walk(fc.CachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// trim the cache suffix first so a FileSuffix of ".tmp" cannot make
		// live cache files match the temp-file pattern.
		if strings.Contains(strings.TrimSuffix(info.Name(), fc.FileSuffix), ".tmp") && info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
		return nil
	})
}

// get cached file name. it's md5 encoded.
func (fc *FileCache) getCacheFileName(key string) string {
	m := md5.New()
	io.WriteString(m, key)
	keyMd5 := hex.EncodeToString(m.Sum(nil))
	cachePath := fc.CachePath
	switch fc.DirectoryLevel {
	case 2:
		cachePath = filepath.Join(cachePath, keyMd5[0:2], keyMd5[2:4])
	case 1:
		cachePath = filepath.Join(cachePath, keyMd5[0:2])
	}

	if ok, _ := exists(cachePath); !ok { // todo : error handle
		_ = os.MkdirAll(cachePath, os.ModePerm) // todo : error handle
	}

	return filepath.Join(cachePath, fmt.Sprintf("%s%s", keyMd5, fc.FileSuffix))
}

// Get value from file cache.
// if non-exist, expired or unreadable, return empty string.
func (fc *FileCache) Get(key string) interface{} {
	item, exist, err := fc.getItem(key)
	if err != nil || !exist {
		return ""
	}
	return item.Data
}

// getItem reads and decodes the cached item for key. exist reports whether a
// live (non-expired) item was found; err reports read or decode failures.
func (fc *FileCache) getItem(key string) (item FileCacheItem, exist bool, err error) {
	fileData, err := FileGetContents(fc.getCacheFileName(key))
	if err != nil {
		if os.IsNotExist(err) {
			return item, false, nil
		}
		return item, false, err
	}
	if err = GobDecode(fileData, &item); err != nil {
		return item, false, err
	}
	if item.Expired.Before(time.Now()) {
		return item, false, nil
	}
	return item, true, nil
}

// GetMulti gets values from file cache.
// if non-exist or expired, return empty string.
func (fc *FileCache) GetMulti(keys []string) []interface{} {
	var rc []interface{}
	for _, key := range keys {
		rc = append(rc, fc.Get(key))
	}
	return rc
}

// Put value into file cache.
// timeout means how long to keep this file.
// if timeout is 0, cache this item forever.
func (fc *FileCache) Put(key string, val interface{}, timeout time.Duration) error {
	gob.Register(val)

	item := FileCacheItem{Data: val}
	if timeout == 0 {
		item.Expired = time.Now().Add(foreverDuration)
	} else {
		item.Expired = time.Now().Add(timeout)
	}
	item.Lastaccess = time.Now()
	data, err := GobEncode(item)
	if err != nil {
		return err
	}
	fc.lock.Lock()
	defer fc.lock.Unlock()
	return FilePutContents(fc.getCacheFileName(key), data)
}

// Delete file cache value.
func (fc *FileCache) Delete(key string) error {
	fc.lock.Lock()
	defer fc.lock.Unlock()
	filename := fc.getCacheFileName(key)
	if ok, _ := exists(filename); ok {
		return os.Remove(filename)
	}
	return nil
}

// Incr increases the cached counter by 1.
// it is equivalent to IncrBy(key, 1), discarding the new value.
func (fc *FileCache) Incr(key string) error {
	_, err := fc.IncrBy(key, 1)
	return err
}

// IncrBy increases the cached counter by increment and returns the new value
// as int64. increment must be >= 0 (use DecrBy to decrease).
// a missing or expired key is created as 0 (expiring after EmbedExpiry
// seconds, or never when EmbedExpiry is 0) before the increment is applied;
// an existing key keeps its original expiry. a non-integer stored value
// returns an error and is left untouched, and so does a value that can no
// longer be gob-decoded (rewrite it with Put or remove it with Delete).
// writes are serialized in-process only: concurrent processes sharing the
// same cache directory can still lose updates.
func (fc *FileCache) IncrBy(key string, increment int) (int64, error) {
	if increment < 0 {
		return 0, errors.New("increment must be >= 0, use DecrBy to decrease")
	}
	fc.lock.Lock()
	defer fc.lock.Unlock()
	return fc.applyIncrOrDecr(key, increment)
}

// Decr decreases the cached counter by 1.
// it is equivalent to DecrBy(key, 1), discarding the new value.
func (fc *FileCache) Decr(key string) error {
	_, err := fc.DecrBy(key, 1)
	return err
}

// DecrBy decreases the cached counter by decrement and returns the new value
// as int64. decrement must be >= 0 (use IncrBy to increase). the result may
// be negative for signed stored values; unsigned stored values return an
// error instead of going below 0. see IncrBy for the missing-key and
// concurrency semantics.
func (fc *FileCache) DecrBy(key string, decrement int) (int64, error) {
	if decrement < 0 {
		return 0, errors.New("decrement must be >= 0, use IncrBy to increase")
	}
	fc.lock.Lock()
	defer fc.lock.Unlock()
	return fc.applyIncrOrDecr(key, -decrement)
}

// applyDelta adds delta to (or subtracts it from, when negate is true) the
// value stored under key and returns the new value. the caller must hold
// fc.lock and guarantee delta >= 0.
func (fc *FileCache) applyIncrOrDecr(key string, diffValue int) (int64, error) {
	item, exist, err := fc.getItem(key)
	if err != nil {
		return 0, err
	}

	if !exist || item.Expired.Before(time.Now()) {
		item = FileCacheItem{Data: int64(0)}
		if fc.EmbedExpiry > 0 {
			item.Expired = time.Now().Add(time.Duration(fc.EmbedExpiry) * time.Second)
		} else {
			item.Expired = time.Now().Add(foreverDuration)
		}
	}

	updated, err := calculateIncrOrDecr(item.Data, diffValue)

	switch val := item.Data.(type) {
	case int64:
		if err != nil {
			return val, err
		}
		item.Data = updated

	default:
		if err != nil {
			currValOrZeroInt64, _ := toInt64(item.Data)
			return currValOrZeroInt64, err
		}

		item = FileCacheItem{Data: updated, Expired: item.Expired, Lastaccess: item.Lastaccess}
	}

	item.Lastaccess = time.Now()
	data, err := GobEncode(item)
	if err != nil {
		return 0, err
	}
	if err = FilePutContents(fc.getCacheFileName(key), data); err != nil {
		return 0, err
	}
	return updated, nil
}

// IsExist check value is exist.
func (fc *FileCache) IsExist(key string) bool {
	ret, _ := exists(fc.getCacheFileName(key))
	return ret
}

// ClearAll will clean cached files.
// not implemented.
func (fc *FileCache) ClearAll() error {
	return nil
}

// check file exist.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// FileGetContents Get bytes from file.
func FileGetContents(filename string) (data []byte, e error) {
	return ioutil.ReadFile(filename)
}

// FilePutContents Put bytes to file.
// the write is atomic: content goes to a temp file in the same directory,
// which is then renamed over the target, so readers never see a torn write.
func FilePutContents(filename string, content []byte) error {
	tmp, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename)+".tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err = os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err = os.Rename(tmpName, filename); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// GobEncode Gob encodes file cache item.
func GobEncode(data interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

// GobDecode Gob decodes file cache item.
func GobDecode(data []byte, to *FileCacheItem) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(&to)
}

func init() {
	// register the counter types up front so gob can decode a
	// FileCacheItem.Data written by another process.
	gob.Register(int(0))
	gob.Register(int32(0))
	gob.Register(int64(0))
	gob.Register(uint(0))
	gob.Register(uint32(0))
	gob.Register(uint64(0))

	Register("file", NewFileCache)
}
