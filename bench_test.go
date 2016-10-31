package dsdbench

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/docker/docker/pkg/stringid"
)

func BenchmarkCreateEmptyLayer(b *testing.B) {
	ls, err := getLayerStore()
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup(b, ls)

	emptyTar, err := TarFromFiles()
	if err != nil {
		b.Fatal("Failed to create tar bytes")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l, err := ls.Register(bytes.NewReader(emptyTar), "")
		if err != nil {
			b.Fatalf("Failed to register new layer: %+v", err)
		}
		b.StopTimer()
		if _, err := ls.Release(l); err != nil {
			b.Fatalf("Failed to release layer: %+v", err)
		}
		b.StartTimer()
	}
}

func BenchmarkGetSingleBaseMount(b *testing.B) {
	b.StopTimer()
	ls, err := getLayerStore()
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup(b, ls)

	l1Init := InitWithFiles(
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
	)

	l, err := CreateLayerChain(ls, l1Init)
	if err != nil {
		b.Fatalf("Failed to create layer chain: %+v", err)
	}

	containerID := stringid.GenerateRandomID()
	rw, err := ls.CreateRWLayer(containerID, l.ChainID(), "", nil, nil)
	if err != nil {
		b.Fatalf("Failed to create rw layer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		_, err := rw.Mount("")
		if err != nil {
			b.Fatalf("Mount error: %v", err)
		}
		b.StopTimer()
		if err := rw.Unmount(); err != nil {
			b.Fatalf("Unmount error: %v", err)
		}
	}
}

func BenchmarkGet20BaseMount(b *testing.B) {
	benchmarkGetBaseMountWithDepth(b, 20)
}

func BenchmarkGet50BaseMount(b *testing.B) {
	benchmarkGetBaseMountWithDepth(b, 50)
}

func BenchmarkGet100BaseMount(b *testing.B) {
	benchmarkGetBaseMountWithDepth(b, 100)
}

func benchmarkGetBaseMountWithDepth(b *testing.B, depth int) {
	b.StopTimer()
	ls, err := getLayerStore()
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup(b, ls)

	inits := make([]LayerInit, depth)
	inits[0] = InitWithFiles(
		CreateDirectory("/etc", 0755),
		NewTestFile("/etc/hosts", []byte("mydomain 10.0.0.1"), 0644),
		NewTestFile("/etc/profile", []byte("PATH=/usr/bin"), 0644),
		CreateDirectory("/testfiles", 0755),
	)
	for i := 1; i < depth; i++ {
		inits[i] = InitWithFiles(
			NewTestFile(fmt.Sprintf("/testfiles/t-%d", i), []byte("irrelevant data"), 0644),
		)
	}

	l, err := CreateLayerChain(ls, inits...)
	if err != nil {
		b.Fatalf("Failed to create layer chain: %+v", err)
	}

	containerID := stringid.GenerateRandomID()
	rw, err := ls.CreateRWLayer(containerID, l.ChainID(), "", nil, nil)
	if err != nil {
		b.Fatalf("Failed to create rw layer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		_, err := rw.Mount("")
		if err != nil {
			b.Fatalf("Mount error: %v", err)
		}
		b.StopTimer()
		if err := rw.Unmount(); err != nil {
			b.Fatalf("Unmount error: %v", err)
		}
	}
}
