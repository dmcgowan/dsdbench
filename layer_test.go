package dsdbench

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/reexec"

	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	_ "github.com/docker/docker/daemon/graphdriver/devmapper"
	_ "github.com/docker/docker/daemon/graphdriver/overlay"
	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
)

func init() {
	reexec.Init()
}

func cleanup(t testing.TB, ls layer.Store) {
	if err := ls.Cleanup(); err != nil {
		t.Logf("cleanup error: %v", err)
	}
}

// simpleLayersTest creates a layer chain made up of the layer init
// functions and compares it with a flat directory with all the
// layer initilizers applied.
func simpleLayerTest(t *testing.T, layers ...LayerInit) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l, err := CreateLayerChain(ls, layers...)
	if err != nil {
		t.Fatalf("Failed to create layer chain: %+v", err)
	}

	if err := CheckLayer(ls, l.ChainID(), layers...); err != nil {
		t.Fatalf("Layer check failure: %+v", err)
	}

	if _, err := ls.Release(l); err != nil {
		t.Fatal(err)
	}
}

func TestLayerCreate(t *testing.T) {
	l1Init := InitWithFiles(
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
	)
	l2Init := InitWithFiles(
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.120"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0666),
		CreateDirectory("/root", 0700),
		NewTestFile("/root/.bashrc", []byte("PATH=/usr/sbin:/usr/bin"), 0644),
	)

	simpleLayerTest(t, l1Init, l2Init)
}

func TestFileDeletion(t *testing.T) {
	l1Init := InitWithFiles(
		CreateDirectory("/test/somedir", 0755),
	)
	l2Init := InitWithFiles(
		NewTestFile("/test/a", []byte{}, 0644),
		NewTestFile("/test/b", []byte{}, 0644),
		CreateDirectory("/test/otherdir", 0755),
		NewTestFile("/test/otherdir/.empty", []byte{}, 0644),
	)
	l3Init := InitWithFiles(
		RemoveFile("/test/a"),
		RemoveFile("/test/b"),
		RemoveFile("/test/otherdir"),
	)

	simpleLayerTest(t, l1Init, l2Init, l3Init)
}

func TestDirectoryReplace(t *testing.T) {
	l1Init := InitWithFiles(
		CreateDirectory("/test/something", 0755),
		NewTestFile("/test/something/f1", []byte{'1'}, 0644),
		NewTestFile("/test/something/f2", []byte{'1'}, 0644),
	)
	l2Init := InitWithFiles(
		RemoveFile("/test/something"),
		NewTestFile("/test/something", []byte("something new!"), 0644),
	)

	simpleLayerTest(t, l1Init, l2Init)
}

func TestTarRegister(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	files1 := []FileApplier{
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
	}
	files2 := []FileApplier{
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.2"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0666),
		CreateDirectory("/root", 0700),
		NewTestFile("/root/.bashrc", []byte("PATH=/usr/sbin:/usr/bin"), 0644),
	}

	tar1, err := TarFromFiles(files1...)
	if err != nil {
		t.Fatal(err)
	}

	tar2, err := TarFromFiles(files2...)
	if err != nil {
		t.Fatal(err)
	}

	layer1, err := ls.Register(bytes.NewReader(tar1), "")
	if err != nil {
		t.Fatal(err)
	}

	layer2, err := ls.Register(bytes.NewReader(tar2), layer1.ChainID())
	if err != nil {
		t.Fatal(err)
	}

	if err := CheckLayer(ls, layer2.ChainID(), InitWithFiles(append(files1, files2...)...)); err != nil {
		t.Fatalf("Layer check failure: %+v", err)
	}
}

func TestMount1to125Layers(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	// Layer store limits at 125
	max := 125
	inits := make([]LayerInit, max)
	inits[0] = InitWithFiles(
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
		CreateDirectory("/testfiles", 0755),
	)

	for i := 1; i < max; i++ {
		inits[i] = InitWithFiles(
			NewTestFile(fmt.Sprintf("/testfiles/t-%d", i), []byte("irrelevant data"), 0644),
		)
	}

	var l layer.Layer
	var parentID layer.ChainID
	for i, lf := range inits {
		previous := l
		l, err = CreateLayer(ls, parentID, lf)
		if err != nil {
			t.Fatalf("failed to create layer %d: %+v", i+1, err)
		}
		parentID = l.ChainID()

		if err := CheckLayer(ls, l.ChainID(), inits[:i+1]...); err != nil {
			t.Fatalf("check layer %d failed: %+v", i+1, err)
		}

		if previous != nil {
			if _, err = ls.Release(previous); err != nil {
				t.Fatalf("layer %d release error: %+v", i+1, err)
			}
		}
	}

	if _, err = ls.Release(l); err != nil {
		t.Fatalf("layer %d release error: %+v", max, err)
	}
}

// TODO: Run basic POSIX operations tests
func TestPosix(t *testing.T) {
	t.Skip("Not implemented")
}
