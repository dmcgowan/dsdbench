package dsdbench

import (
	"bytes"
	"testing"

	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Init()
}

func cleanup(t testing.TB, ls layer.Store) {
	if err := ls.Cleanup(); err != nil {
		t.Logf("cleanup error: %v", err)
	}
}

func TestLayerCreate(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l1Init := InitWithFiles([]FileApplier{
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
	}...)
	l2Init := InitWithFiles([]FileApplier{
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.120"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0666),
		CreateDirectory("/root", 0700),
		NewTestFile("/root/.bashrc", []byte("PATH=/usr/sbin:/usr/bin"), 0644),
	}...)

	l, err := CreateLayerChain(ls, l1Init, l2Init)
	if err != nil {
		t.Fatalf("Failed to create layer chain: %+v", err)
	}

	if err := CheckLayer(ls, l.ChainID(), l1Init, l2Init); err != nil {
		t.Fatalf("Layer check failure: %+v", err)
	}

	if _, err := ls.Release(l); err != nil {
		t.Fatal(err)
	}
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

func TestFileDeletion(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l1Init := InitWithFiles([]FileApplier{
		CreateDirectory("/test/somedir", 0755),
	}...)
	l2Init := InitWithFiles([]FileApplier{
		NewTestFile("/test/a", []byte{}, 0644),
		NewTestFile("/test/b", []byte{}, 0644),
		CreateDirectory("/test/otherdir", 0755),
		NewTestFile("/test/otherdir/.empty", []byte{}, 0644),
	}...)
	l3Init := InitWithFiles([]FileApplier{
		RemoveFile("/test/a"),
		RemoveFile("/test/b"),
		RemoveFile("/test/otherdir"),
	}...)

	l, err := CreateLayerChain(ls, l1Init, l2Init, l3Init)
	if err != nil {
		t.Fatalf("Failed to create layer chain: %+v", err)
	}

	if err := CheckLayer(ls, l.ChainID(), l1Init, l2Init, l3Init); err != nil {
		t.Fatalf("Layer check failure: %+v", err)
	}

	if _, err := ls.Release(l); err != nil {
		t.Fatal(err)
	}
}

// TODO: Run basic POSIX operations tests
func TestPosix(t *testing.T) {
	t.Skip("Not implemented")
}

// TODO: Benchmarks!
