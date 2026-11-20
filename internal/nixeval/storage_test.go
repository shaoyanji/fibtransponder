package nixeval

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/shaoyanji/fibtransponder/internal/deltaqueue"
)

func TestGreedyTerms(t *testing.T) {
	fibs := []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}
	indexes := make(map[int]int)
	for i, f := range fibs {
		indexes[f] = i
	}
	for n := 0; n <= 100; n++ {
		sum, previous := 0, len(fibs)+1
		for _, term := range deltaqueue.ZeckendorfTerms(n) {
			index, ok := indexes[term]
			if !ok || previous-index <= 1 {
				t.Fatalf("noncanonical terms for %d", n)
			}
			previous = index
			sum += term
		}
		if sum != n {
			t.Fatalf("terms sum %d != %d", sum, n)
		}
	}
	if !reflect.DeepEqual(deltaqueue.ZeckendorfTerms(12), []int{8, 3, 1}) {
		t.Fatal("wrong terms for 12")
	}
}

func TestUninitialisedGitlinkDoesNotHashParentDirtiness(t *testing.T) {
	x := newFixture(t, 0)
	if err := os.Mkdir(filepath.Join(x.repo, "submodule"), 0700); err != nil {
		t.Fatal(err)
	}
	git(t, x.repo, "update-index", "--add", "--cacheinfo", "160000,1111111111111111111111111111111111111111,submodule")
	x.get()
	epoch := x.status().Epoch
	x.write("00.doc", "edited")
	x.get()
	if x.status().Epoch != epoch {
		t.Fatal("uninitialised gitlink hashed parent dirtiness")
	}
	x.write("core.nix", "edited")
	x.get()
	if x.status().Epoch == epoch {
		t.Fatal("core edit failed to leap")
	}
}

func TestLockLiveOwnerCancellationAndProcessExit(t *testing.T) {
	if dir := os.Getenv("NIXEVAL_LOCK_CHILD"); dir != "" {
		w := worker{dir: dir}
		if _, err := w.lock(context.Background()); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	w := worker{dir: t.TempDir()}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockLiveOwnerCancellationAndProcessExit$")
	cmd.Env = append(os.Environ(), "NIXEVAL_LOCK_CHILD="+w.dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %s %v", out, err)
	}
	unlock, err := w.lock(context.Background())
	if err != nil {
		t.Fatal("child retained lock", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if release, err := w.lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		unlock()
		t.Fatal("live lock stolen", err)
	}
	unlock()
	release, err := w.lock(context.Background())
	if err != nil {
		t.Fatal("lock not released", err)
	}
	release()
}
