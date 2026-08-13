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
	"math"
	"os"
	"sync"
	"testing"
	"time"
)

func TestCacheIncr(t *testing.T) {
	bm, err := NewCache("memory", `{"interval":20}`)
	if err != nil {
		t.Error("init err")
	}
	//timeoutDuration := 10 * time.Second

	bm.Put("edwardhey", 0, time.Second*20)
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			bm.Incr("edwardhey")
		}()
	}
	wg.Wait()
	if bm.Get("edwardhey").(int64) != 10 {
		t.Error("Incr err")
	}
}

func TestCacheIncrBy(t *testing.T) {
	bm, err := NewCache("memory", `{"interval":20}`)
	if err != nil {
		t.Error("init err")
	}

	bm.Put("edwardhey", 0, time.Second*20)
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(x int) {
			defer wg.Done()
			bm.IncrBy("edwardhey", x)
		}(i + 1)
	}
	wg.Wait()
	// 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
	if bm.Get("edwardhey").(int64) != 55 {
		t.Error("Incr err")
	}
}

func TestCache(t *testing.T) {
	bm, err := NewCache("memory", `{"interval":20}`)
	if err != nil {
		t.Error("init err")
	}
	timeoutDuration := 10 * time.Second
	if err = bm.Put("astaxie", 1, timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie") {
		t.Error("check err")
	}

	if v := bm.Get("astaxie"); v.(int) != 1 {
		t.Error("get err")
	}

	time.Sleep(10 * time.Second)

	if bm.IsExist("astaxie") {
		t.Error("check err")
	}

	if err = bm.Put("astaxie", 1, timeoutDuration); err != nil {
		t.Error("set Error", err)
	}

	if err = bm.Incr("astaxie"); err != nil {
		t.Error("Incr Error", err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 2 {
		t.Error("get err")
	}

	if v, err := bm.IncrBy("astaxie", 3); err != nil || v != 5 {
		t.Error("IncrBy Error", v, err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 5 {
		t.Error("get err")
	}

	if v, err := bm.DecrBy("astaxie", 3); err != nil || v != 2 {
		t.Error("DecrBy Error", v, err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 2 {
		t.Error("get err")
	}

	if err = bm.Decr("astaxie"); err != nil {
		t.Error("Decr Error", err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 1 {
		t.Error("get err")
	}
	bm.Delete("astaxie")
	if bm.IsExist("astaxie") {
		t.Error("delete err")
	}

	//test GetMulti
	if err = bm.Put("astaxie", "author", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie") {
		t.Error("check err")
	}
	if v := bm.Get("astaxie"); v.(string) != "author" {
		t.Error("get err")
	}

	if err = bm.Put("astaxie1", "author1", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie1") {
		t.Error("check err")
	}

	vv := bm.GetMulti([]string{"astaxie", "astaxie1"})
	if len(vv) != 2 {
		t.Error("GetMulti ERROR")
	}
	if vv[0].(string) != "author" {
		t.Error("GetMulti ERROR")
	}
	if vv[1].(string) != "author1" {
		t.Error("GetMulti ERROR")
	}
}

func TestFileCache(t *testing.T) {
	bm, err := NewCache("file", `{"CachePath":"cache","FileSuffix":".bin","DirectoryLevel":"2","EmbedExpiry":"0"}`)
	if err != nil {
		t.Error("init err")
	}
	timeoutDuration := 10 * time.Second
	if err = bm.Put("astaxie", 1, timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie") {
		t.Error("check err")
	}

	if v := bm.Get("astaxie"); v.(int) != 1 {
		t.Error("get err")
	}

	if err = bm.Incr("astaxie"); err != nil {
		t.Error("Incr Error", err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 2 {
		t.Error("get err")
	}

	if v, err := bm.IncrBy("astaxie", 3); err != nil || v != 5 {
		t.Error("IncrBy Error", v, err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 5 {
		t.Error("get err")
	}

	if v, err := bm.DecrBy("astaxie", 3); err != nil || v != 2 {
		t.Error("DecrBy Error", v, err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 2 {
		t.Error("get err")
	}

	if err = bm.Decr("astaxie"); err != nil {
		t.Error("Decr Error", err)
	}

	if v := bm.Get("astaxie"); v.(int64) != 1 {
		t.Error("get err")
	}
	bm.Delete("astaxie")
	if bm.IsExist("astaxie") {
		t.Error("delete err")
	}

	//test string
	if err = bm.Put("astaxie", "author", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie") {
		t.Error("check err")
	}
	if v := bm.Get("astaxie"); v.(string) != "author" {
		t.Error("get err")
	}

	//test GetMulti
	if err = bm.Put("astaxie1", "author1", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie1") {
		t.Error("check err")
	}

	vv := bm.GetMulti([]string{"astaxie", "astaxie1"})
	if len(vv) != 2 {
		t.Error("GetMulti ERROR")
	}
	if vv[0].(string) != "author" {
		t.Error("GetMulti ERROR")
	}
	if vv[1].(string) != "author1" {
		t.Error("GetMulti ERROR")
	}

	os.RemoveAll("cache")
}

func TestMemoryCounterGuards(t *testing.T) {
	bm, err := NewCache("memory", `{"interval":20}`)
	if err != nil {
		t.Fatal("init err")
	}

	// negative deltas are rejected in both directions
	if _, err := bm.IncrBy("neg", -1); err == nil {
		t.Error("IncrBy with negative increment should fail")
	}
	if _, err := bm.DecrBy("neg", -1); err == nil {
		t.Error("DecrBy with negative decrement should fail")
	}

	// unsigned underflow: 0 < val < decrement must error and keep the value
	bm.Put("u", uint(3), 0)
	if v, err := bm.DecrBy("u", 5); err != nil || v != -2 {
		t.Error("DecrBy(uint(3), 5) should return -2")
	}
	if v := bm.Get("u"); v.(int64) != -2 {
		t.Error("value shoud be -2 after DecrBy, got", v)
	}
	if v, err := bm.IncrBy("u", 2); err != nil || v != 0 {
		t.Error("DecrBy(-2, 2) should reach 0, got", v, err)
	}

	// int32 overflow guard
	bm.Put("i32", int32(1), 0)
	if v, err := bm.IncrBy("i32", math.MaxInt32); err != nil || v != math.MaxInt32+1 {
		t.Error("IncrBy(1, MaxInt32) shoud be MaxInt32+1, got", v, err)
	}

	// uint64 result above MaxInt64 is not representable in the return
	bm.Put("u64", uint64(math.MaxInt64), 0)
	if v := bm.Get("u64"); v.(uint64) != math.MaxInt64 {
		t.Error("value shoud be MaxInt64, got", v)
	} else if x, err := toInt64(v); err != nil || x != math.MaxInt64 {
		t.Error("value shoud be MaxInt64, got", v, err)
	}

	if v, err := bm.IncrBy("u64", 1); err == nil || v != math.MaxInt64 {
		t.Error("IncrBy overflowing int64 should fail and return previous value (MaxInt64), got", v, err)
	}

	// signed values may go negative
	bm.Put("i", 5, 0)
	if v, err := bm.DecrBy("i", 8); err != nil || v != -3 {
		t.Error("DecrBy(5, 8) should return -3, got", v, err)
	}

	// missing keys are created as 0 before the delta is applied
	if v, err := bm.IncrBy("fresh", 7); err != nil || v != 7 {
		t.Error("IncrBy on missing key should return 7, got", v, err)
	}
	if v := bm.Get("fresh"); v.(int64) != 7 {
		t.Error("auto-init value err, got", v)
	}
	if v, err := bm.DecrBy("fresh2", 3); err != nil || v != -3 {
		t.Error("DecrBy on missing key should return -3, got", v, err)
	}

	// expired keys behave like missing keys
	bm.Put("exp", 100, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if v, err := bm.IncrBy("exp", 2); err != nil || v != 2 {
		t.Error("IncrBy on expired key should restart from 0, got", v, err)
	}

	// non-numeric values error out without being overwritten
	bm.Put("str", "author", 0)
	if _, err := bm.IncrBy("str", 1); err == nil {
		t.Error("IncrBy on string value should fail")
	}
	if v := bm.Get("str"); v.(string) != "author" {
		t.Error("string value must stay untouched, got", v)
	}
}

func TestFileCacheCounters(t *testing.T) {
	cachePath := "cache_counters"
	defer os.RemoveAll(cachePath)
	bm, err := NewCache("file", `{"CachePath":"`+cachePath+`","FileSuffix":".bin","DirectoryLevel":"2","EmbedExpiry":"0"}`)
	if err != nil {
		t.Fatal("init err")
	}

	// negative deltas are rejected
	if _, err := bm.IncrBy("neg", -1); err == nil {
		t.Error("IncrBy with negative increment should fail")
	}
	if _, err := bm.DecrBy("neg", -1); err == nil {
		t.Error("DecrBy with negative decrement should fail")
	}

	// missing keys are created as 0 before the delta is applied
	if v, err := bm.IncrBy("fresh", 7); err != nil || v != 7 {
		t.Error("IncrBy on missing key should return 7, got", v, err)
	}
	if v := bm.Get("fresh"); v.(int64) != 7 {
		t.Error("auto-init value err, got", v)
	}

	// signed values may go negative (the old clamp at 0 is gone)
	if err := bm.Put("i", 5, 10*time.Second); err != nil {
		t.Fatal("set Error", err)
	}
	if v, err := bm.DecrBy("i", 10); err != nil || v != -5 {
		t.Error("DecrBy(5, 10) should return -5, got", v, err)
	}
	if v := bm.Get("i"); v.(int64) != -5 {
		t.Error("get err, got", v)
	}

	// non-numeric values error out without being overwritten
	if err := bm.Put("str", "author", 10*time.Second); err != nil {
		t.Fatal("set Error", err)
	}
	if _, err := bm.IncrBy("str", 1); err == nil {
		t.Error("IncrBy on string value should fail")
	}
	if v := bm.Get("str"); v.(string) != "author" {
		t.Error("string value must stay untouched, got", v)
	}

	// the key's original expiry is preserved across increments
	fc := bm.(*FileCache)
	if err := bm.Put("ttl", 1, 10*time.Second); err != nil {
		t.Fatal("set Error", err)
	}
	before, exist, err := fc.getItem("ttl")
	if err != nil || !exist {
		t.Fatal("getItem err", exist, err)
	}
	if _, err := bm.IncrBy("ttl", 1); err != nil {
		t.Fatal("IncrBy err", err)
	}
	after, exist, err := fc.getItem("ttl")
	if err != nil || !exist {
		t.Fatal("getItem err", exist, err)
	}
	if !after.Expired.Equal(before.Expired) {
		t.Error("IncrBy must keep the original expiry, got", before.Expired, "->", after.Expired)
	}

	// concurrent increments are serialized by the adapter's mutex
	if err := bm.Put("conc", 0, 20*time.Second); err != nil {
		t.Fatal("set Error", err)
	}
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(x int) {
			defer wg.Done()
			bm.IncrBy("conc", x)
		}(i + 1)
	}
	wg.Wait()
	// 1 + 2 + ... + 10
	if v := bm.Get("conc"); v.(int64) != 55 {
		t.Error("concurrent IncrBy err, got", v)
	}
}
