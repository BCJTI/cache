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

package memcache

import (
	"strconv"
	"testing"
	"time"

	"github.com/bcjti/cache"
	_ "github.com/bradfitz/gomemcache/memcache"
)

func TestMemcacheCache(t *testing.T) {
	bm, err := cache.NewCache("memcache", `{"conn": "127.0.0.1:11211"}`)
	if err != nil {
		t.Error("init err")
	}
	timeoutDuration := 10 * time.Second
	if err = bm.Put("astaxie", "1", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}
	if !bm.IsExist("astaxie") {
		t.Error("check err")
	}

	time.Sleep(11 * time.Second)

	if bm.IsExist("astaxie") {
		t.Error("check err")
	}
	if err = bm.Put("astaxie", "1", timeoutDuration); err != nil {
		t.Error("set Error", err)
	}

	if v, err := strconv.Atoi(string(bm.Get("astaxie").([]byte))); err != nil || v != 1 {
		t.Error("get err")
	}

	if err = bm.Incr("astaxie"); err != nil {
		t.Error("Incr Error", err)
	}

	if v, err := strconv.Atoi(string(bm.Get("astaxie").([]byte))); err != nil || v != 2 {
		t.Error("get err")
	}

	if v, err := bm.IncrBy("astaxie", 3); err != nil || v != 5 {
		t.Error("IncrBy Error", v, err)
	}

	if v, err := strconv.Atoi(string(bm.Get("astaxie").([]byte))); err != nil || v != 5 {
		t.Error("get err")
	}

	if v, err := bm.DecrBy("astaxie", 3); err != nil || v != 2 {
		t.Error("DecrBy Error", v, err)
	}

	if v, err := strconv.Atoi(string(bm.Get("astaxie").([]byte))); err != nil || v != 2 {
		t.Error("get err")
	}

	if err = bm.Decr("astaxie"); err != nil {
		t.Error("Decr Error", err)
	}

	if v, err := strconv.Atoi(string(bm.Get("astaxie").([]byte))); err != nil || v != 1 {
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

	if v := bm.Get("astaxie").([]byte); string(v) != "author" {
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
	if string(vv[0].([]byte)) != "author" && string(vv[0].([]byte)) != "author1" {
		t.Error("GetMulti ERROR")
	}
	if string(vv[1].([]byte)) != "author1" && string(vv[1].([]byte)) != "author" {
		t.Error("GetMulti ERROR")
	}

	// test clear all
	if err = bm.ClearAll(); err != nil {
		t.Error("clear all err")
	}
}

func TestMemcacheSetNotSupported(t *testing.T) {
	bm, err := cache.NewCache("memcache", `{"conn": "127.0.0.1:11211"}`)
	if err != nil {
		t.Fatal("init err", err)
	}
	if _, err := bm.SAdd("set", "a"); err != cache.ErrNotSupported {
		t.Fatalf("SAdd err = %v, want ErrNotSupported", err)
	}
	if _, err := bm.SRem("set", "a"); err != cache.ErrNotSupported {
		t.Fatalf("SRem err = %v, want ErrNotSupported", err)
	}
	if _, err := bm.SMembers("set"); err != cache.ErrNotSupported {
		t.Fatalf("SMembers err = %v, want ErrNotSupported", err)
	}
}

func TestMemcacheExpireAndPutIfExists(t *testing.T) {
	bm, err := cache.NewCache("memcache", `{"conn": "127.0.0.1:11211"}`)
	if err != nil {
		t.Fatal("init err", err)
	}
	bm.Delete("expire-key")
	if ok, err := bm.PutIfExists("expire-key", "v1", time.Minute); err != nil || ok {
		t.Fatalf("PutIfExists(missing) = %v, %v; want false, nil", ok, err)
	}
	if err := bm.Put("expire-key", "v1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if ok, err := bm.PutIfExists("expire-key", "v2", time.Minute); err != nil || !ok {
		t.Fatalf("PutIfExists(existing) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := bm.Expire("expire-key", time.Second); err != nil || !ok {
		t.Fatalf("Expire = %v, %v; want true, nil", ok, err)
	}
	if ok, err := bm.Expire("expire-missing", time.Second); err != nil || ok {
		t.Fatalf("Expire(missing) = %v, %v; want false, nil", ok, err)
	}
	time.Sleep(2 * time.Second)
	if bm.IsExist("expire-key") {
		t.Fatal("key still exists after Expire")
	}
}
