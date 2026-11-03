package cache

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

func newTestFileCache(t *testing.T) Cache {
	dir, err := ioutil.TempDir("", "cache-set-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	bm, err := NewCache("file", fmt.Sprintf(`{"CachePath":%q,"FileSuffix":".bin","DirectoryLevel":"2","EmbedExpiry":"0"}`, dir))
	if err != nil {
		t.Fatal(err)
	}
	return bm
}

func setAdapters(t *testing.T) map[string]Cache {
	mem, err := NewCache("memory", `{"interval":1}`)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]Cache{"memory": mem, "file": newTestFileCache(t)}
}

func mustMembers(t *testing.T, bm Cache, key string, want ...string) {
	t.Helper()
	got, err := bm.SMembers(key)
	if err != nil {
		t.Fatalf("SMembers(%q): %v", key, err)
	}
	sort.Strings(got)
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SMembers(%q) = %v, want %v", key, got, want)
	}
}

func TestSetOperations(t *testing.T) {
	for name, bm := range setAdapters(t) {
		t.Run(name, func(t *testing.T) {
			mustMembers(t, bm, "set")

			if n, err := bm.SAdd("set", "a", "b", "a"); err != nil || n != 2 {
				t.Fatalf("SAdd = %d, %v; want 2, nil", n, err)
			}
			if n, err := bm.SAdd("set", "b", "c"); err != nil || n != 1 {
				t.Fatalf("SAdd = %d, %v; want 1, nil", n, err)
			}
			mustMembers(t, bm, "set", "a", "b", "c")
			if !bm.IsExist("set") {
				t.Fatal("IsExist(set) = false after SAdd")
			}

			if n, err := bm.SRem("set", "a", "x"); err != nil || n != 1 {
				t.Fatalf("SRem = %d, %v; want 1, nil", n, err)
			}
			mustMembers(t, bm, "set", "b", "c")

			if n, err := bm.SRem("set", "b", "c"); err != nil || n != 2 {
				t.Fatalf("SRem = %d, %v; want 2, nil", n, err)
			}
			mustMembers(t, bm, "set")
			if bm.IsExist("set") {
				t.Fatal("IsExist(set) = true after removing every member")
			}

			if n, err := bm.SRem("missing", "a"); err != nil || n != 0 {
				t.Fatalf("SRem(missing) = %d, %v; want 0, nil", n, err)
			}
			if _, err := bm.SAdd("set"); err == nil {
				t.Fatal("SAdd without members: want error")
			}
			if _, err := bm.SRem("set"); err == nil {
				t.Fatal("SRem without members: want error")
			}

			if err := bm.Put("plain", "v", time.Minute); err != nil {
				t.Fatal(err)
			}
			if _, err := bm.SAdd("plain", "a"); err == nil {
				t.Fatal("SAdd on a non-set key: want error")
			}
			if _, err := bm.SRem("plain", "a"); err == nil {
				t.Fatal("SRem on a non-set key: want error")
			}
			if _, err := bm.SMembers("plain"); err == nil {
				t.Fatal("SMembers on a non-set key: want error")
			}
		})
	}
}

func TestSetConcurrentAdd(t *testing.T) {
	for name, bm := range setAdapters(t) {
		t.Run(name, func(t *testing.T) {
			const n = 50
			var wg sync.WaitGroup
			wg.Add(n)
			for i := 0; i < n; i++ {
				go func(i int) {
					defer wg.Done()
					if _, err := bm.SAdd("conc", fmt.Sprint(i)); err != nil {
						t.Error(err)
					}
				}(i)
			}
			wg.Wait()
			got, err := bm.SMembers("conc")
			if err != nil || len(got) != n {
				t.Fatalf("SMembers = %d members, %v; want %d", len(got), err, n)
			}
		})
	}
}

func TestExpire(t *testing.T) {
	for name, bm := range setAdapters(t) {
		t.Run(name, func(t *testing.T) {
			if ok, err := bm.Expire("missing", time.Second); err != nil || ok {
				t.Fatalf("Expire(missing) = %v, %v; want false, nil", ok, err)
			}
			if _, err := bm.Expire("missing", 0); err == nil {
				t.Fatal("Expire with timeout 0: want error")
			}

			if _, err := bm.SAdd("set", "a"); err != nil {
				t.Fatal(err)
			}
			if err := bm.Put("plain", "v", 0); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"set", "plain"} {
				if ok, err := bm.Expire(key, 300*time.Millisecond); err != nil || !ok {
					t.Fatalf("Expire(%q) = %v, %v; want true, nil", key, ok, err)
				}
			}
			mustMembers(t, bm, "set", "a")

			time.Sleep(500 * time.Millisecond)

			mustMembers(t, bm, "set")
			if v := bm.Get("plain"); v != nil && v != "" {
				t.Fatalf("Get(plain) after expiry = %v", v)
			}
			if ok, err := bm.Expire("plain", time.Second); err != nil || ok {
				t.Fatalf("Expire(expired) = %v, %v; want false, nil", ok, err)
			}
			// an expired set starts over on the next SAdd, without TTL.
			if n, err := bm.SAdd("set", "b"); err != nil || n != 1 {
				t.Fatalf("SAdd after expiry = %d, %v; want 1, nil", n, err)
			}
			mustMembers(t, bm, "set", "b")
		})
	}
}

func TestPutIfExists(t *testing.T) {
	for name, bm := range setAdapters(t) {
		t.Run(name, func(t *testing.T) {
			if ok, err := bm.PutIfExists("k", "v1", time.Minute); err != nil || ok {
				t.Fatalf("PutIfExists(missing) = %v, %v; want false, nil", ok, err)
			}
			if bm.IsExist("k") {
				t.Fatal("PutIfExists created a missing key")
			}
			if _, err := bm.PutIfExists("k", "v1", -time.Second); err == nil {
				t.Fatal("PutIfExists with negative timeout: want error")
			}

			if err := bm.Put("k", "v1", 0); err != nil {
				t.Fatal(err)
			}
			if ok, err := bm.PutIfExists("k", "v2", 300*time.Millisecond); err != nil || !ok {
				t.Fatalf("PutIfExists(existing) = %v, %v; want true, nil", ok, err)
			}
			if v := bm.Get("k"); v != "v2" {
				t.Fatalf("Get(k) = %v, want v2", v)
			}

			time.Sleep(500 * time.Millisecond)

			if ok, err := bm.PutIfExists("k", "v3", 0); err != nil || ok {
				t.Fatalf("PutIfExists(expired) = %v, %v; want false, nil", ok, err)
			}
		})
	}
}

func TestMemoryGetOnSetReturnsNil(t *testing.T) {
	bm, err := NewCache("memory", `{"interval":0}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bm.SAdd("set", "a"); err != nil {
		t.Fatal(err)
	}
	if v := bm.Get("set"); v != nil {
		t.Fatalf("Get on a set = %v, want nil", v)
	}
	if _, err := bm.IncrBy("set", 1); err == nil {
		t.Fatal("IncrBy on a set: want error")
	}
}
