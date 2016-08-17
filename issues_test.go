package dsdbench

import (
	"testing"
	"time"
)

// TestLayerFileUpdate tests the update of a single file in an upper layer
// Known sporadic failure in overlay, possible in all except overlay2 and aufs
// See https://github.com/docker/docker/issues/21555
func TestLayerFileUpdate(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l1Init := InitWithFiles([]FileApplier{
		CreateDirectory("/etc", 0700),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
	}...)
	l2Init := InitWithFiles([]FileApplier{
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.2"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0666),
		CreateDirectory("/root", 0700),
		NewTestFile("/root/.bashrc", []byte("PATH=/usr/sbin:/usr/bin"), 0644),
	}...)

	var sleepTime time.Duration

	// run 5 times to account for sporadic failure
	for i := 0; i < 5; i++ {
		time.Sleep(sleepTime)

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

		// Sleep until next second boundary before running again
		nextTime := time.Now()
		sleepTime = time.Unix(nextTime.Unix()+1, 0).Sub(nextTime)
	}
}

// See https://github.com/docker/docker/issues/25244
func TestRemoveDirectoryInLowerLayer(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l1Init := InitWithFiles([]FileApplier{
		CreateDirectory("/lib", 0700),
		NewTestFile("/lib/hidden", []byte{}, 0644),
	}...)
	l2Init := InitWithFiles([]FileApplier{
		RemoveFile("/lib"),
		CreateDirectory("/lib", 0700),
		NewTestFile("/lib/not-hidden", []byte{}, 0644),
	}...)
	l3Init := InitWithFiles([]FileApplier{
		NewTestFile("/lib/newfile", []byte{}, 0644),
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

// See https://github.com/docker/docker/issues/24309
func TestRemoveAfterCommit(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/docker/issues/12080
func TestUnixDomainSockets(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/docker/issues/19647
func TestDirectoryInodeStability(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/docker/issues/12327
func TestOpenFileInodeStability(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/docker/issues/19082
func TestGetCWD(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/machine/issues/3327
func TestChmod(t *testing.T) {
	t.Skip("Not implemented")
}

// See https://github.com/docker/docker/issues/24913
func TestChown(t *testing.T) {
	t.Skip("Not implemented")
}

// https://github.com/docker/docker/issues/25409
func TestRename(t *testing.T) {
	ls, err := getLayerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, ls)

	l1Init := InitWithFiles([]FileApplier{
		CreateDirectory("/dir1", 0700),
		CreateDirectory("/somefiles", 0700),
		NewTestFile("/somefiles/f1", []byte("was here first!"), 0644),
		NewTestFile("/somefiles/f2", []byte("nothing interesting"), 0644),
	}...)
	l2Init := InitWithFiles([]FileApplier{
		Rename("/dir1", "/dir2"),
		NewTestFile("/somefiles/f1-overwrite", []byte("new content 1"), 0644),
		Rename("/somefiles/f1-overwrite", "/somefiles/f1"),
		Rename("/somefiles/f2", "/somefiles/f3"),
	}...)

	l, err := CreateLayerChain(ls, l1Init, l2Init)
	if err != nil {
		t.Fatalf("Failed to create layer chain: %+v", err)
	}

	if err := CheckLayer(ls, l.ChainID(), l1Init, l2Init); err != nil {
		t.Fatalf("Unexpected layer content: %+v", err)
	}
}