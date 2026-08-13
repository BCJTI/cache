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
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	// DefaultEvery means the clock time of recycling the expired cache items in memory.
	DefaultEvery = 60 // 1 minute
)

// MemoryItem store memory cache item.
type MemoryItem struct {
	val         interface{}
	createdTime time.Time
	lifespan    time.Duration
}

func (mi *MemoryItem) isExpire() bool {
	// 0 means forever
	if mi.lifespan == 0 {
		return false
	}
	return time.Now().Sub(mi.createdTime) > mi.lifespan
}

// MemoryCache is Memory cache adapter.
// it contains a RW locker for safe map storage.
type MemoryCache struct {
	sync.RWMutex
	dur   time.Duration
	items map[string]*MemoryItem
	Every int // run an expiration check Every clock time
}

// NewMemoryCache returns a new MemoryCache.
func NewMemoryCache() Cache {
	cache := MemoryCache{items: make(map[string]*MemoryItem)}
	return &cache
}

// Get cache from memory.
// if non-existed or expired, return nil.
func (bc *MemoryCache) Get(name string) interface{} {
	bc.RLock()
	defer bc.RUnlock()
	if itm, ok := bc.items[name]; ok {
		if itm.isExpire() {
			return nil
		}
		return itm.val
	}
	return nil
}

// GetMulti gets caches from memory.
// if non-existed or expired, return nil.
func (bc *MemoryCache) GetMulti(names []string) []interface{} {
	var rc []interface{}
	for _, name := range names {
		rc = append(rc, bc.Get(name))
	}
	return rc
}

// Put cache to memory.
// if lifespan is 0, it will be forever till restart.
func (bc *MemoryCache) Put(name string, value interface{}, lifespan time.Duration) error {
	bc.Lock()
	defer bc.Unlock()
	bc.items[name] = &MemoryItem{
		val:         value,
		createdTime: time.Now(),
		lifespan:    lifespan,
	}
	return nil
}

// Delete cache in memory.
func (bc *MemoryCache) Delete(name string) error {
	bc.Lock()
	defer bc.Unlock()
	if _, ok := bc.items[name]; !ok {
		return errors.New("key not exist")
	}
	delete(bc.items, name)
	if _, ok := bc.items[name]; ok {
		return errors.New("delete key error")
	}
	return nil
}

// Incr increase cache counter in memory.
// it supports int,int32,int64,uint,uint32,uint64.
func (bc *MemoryCache) Incr(key string) error {
	_, err := bc.IncrBy(key, 1)
	return err
}

// Incr increase cache counter in memory.
// it supports int, int32, int64, uint, uint32 and uint64 stored values; the
// stored value keeps its original type.
func (bc *MemoryCache) IncrBy(key string, increment int) (int64, error) {
	if increment < 0 {
		return 0, errors.New("increment must be >= 0, use DecrBy to decrease")
	}
	return bc.applyIncrOrDecrDiffValue(key, increment)
}

// Decr decreases cache counter in memory by 1.
// it is equivalent to DecrBy(key, 1), discarding the new value.
func (bc *MemoryCache) Decr(key string) error {
	_, err := bc.DecrBy(key, 1)
	return err
}

// DecrBy decreases the cache counter in memory by decrement and returns the
// new value as int64. decrement must be >= 0 (use IncrBy to increase).
// a missing or expired key is created as 0, without expiration, before the
// decrement is applied. unsigned stored values return an error instead of
// going below 0.
func (bc *MemoryCache) DecrBy(key string, decrement int) (int64, error) {
	if decrement < 0 {
		return 0, errors.New("decrement must be >= 0, use IncrBy to increase")
	}
	return bc.applyIncrOrDecrDiffValue(key, -decrement)
}

func (bc *MemoryCache) applyIncrOrDecrDiffValue(key string, diffValue int) (int64, error) {
	bc.Lock()
	defer bc.Unlock()

	itm, ok := bc.items[key]

	if !ok || itm.isExpire() {
		value := int64(0)
		bc.items[key] = &MemoryItem{val: value, createdTime: time.Now(), lifespan: 0}
		itm = bc.items[key]
	}

	updated, err := calculateIncrOrDecr(itm.val, diffValue)

	switch val := itm.val.(type) {
	case int64:
		// handle calculateIncrOrDecr for itm.val int64 type
		if err != nil {
			// return current value and error occurred when applying diffValue
			return val, err
		}

		// apply and return updated value
		itm.val = updated
		return updated, nil
	}

	// handle calculateIncrOrDecr for itm.val not int64 type
	if err != nil {
		// retrieve current value in int64 for (u)int(8,16,32,64) types or zero in int64
		currValOrZeroInt64, _ := toInt64(itm.val)

		// return current value or zero and error occurred when applying diffValue
		return currValOrZeroInt64, err
	}

	// for itm.val not int64 type, recreate MemoryItem record replacing val type to int64 and keeping other data
	bc.items[key] = &MemoryItem{val: updated, createdTime: itm.createdTime, lifespan: itm.lifespan}
	return updated, nil
}

// IsExist check cache exist in memory.
func (bc *MemoryCache) IsExist(name string) bool {
	bc.RLock()
	defer bc.RUnlock()
	if v, ok := bc.items[name]; ok {
		return !v.isExpire()
	}
	return false
}

// ClearAll will delete all cache in memory.
func (bc *MemoryCache) ClearAll() error {
	bc.Lock()
	defer bc.Unlock()
	bc.items = make(map[string]*MemoryItem)
	return nil
}

// StartAndGC start memory cache. it will check expiration in every clock time.
func (bc *MemoryCache) StartAndGC(config string) error {
	var cf map[string]int
	json.Unmarshal([]byte(config), &cf)
	if _, ok := cf["interval"]; !ok {
		cf = make(map[string]int)
		cf["interval"] = DefaultEvery
	}
	dur := time.Duration(cf["interval"]) * time.Second
	bc.Every = cf["interval"]
	bc.dur = dur
	go bc.vacuum()
	return nil
}

// check expiration.
func (bc *MemoryCache) vacuum() {
	bc.RLock()
	every := bc.Every
	bc.RUnlock()

	if every < 1 {
		return
	}
	for {
		<-time.After(bc.dur)
		bc.RLock()
		if bc.items == nil {
			bc.RUnlock()
			return
		}
		bc.RUnlock()
		if keys := bc.expiredKeys(); len(keys) != 0 {
			bc.clearItems(keys)
		}
	}
}

// expiredKeys returns key list which are expired.
func (bc *MemoryCache) expiredKeys() (keys []string) {
	bc.RLock()
	defer bc.RUnlock()
	for key, itm := range bc.items {
		if itm.isExpire() {
			keys = append(keys, key)
		}
	}
	return
}

// clearItems removes the items whose keys are in keys, re-checking expiry
// under the write lock: a key collected by the GC scan may have been
// re-created meanwhile (e.g. auto-initialized by IncrBy/DecrBy) and must
// survive the sweep.
func (bc *MemoryCache) clearItems(keys []string) {
	bc.Lock()
	defer bc.Unlock()
	for _, key := range keys {
		if itm, ok := bc.items[key]; ok && itm.isExpire() {
			delete(bc.items, key)
		}
	}
}

func init() {
	Register("memory", NewMemoryCache)
}
