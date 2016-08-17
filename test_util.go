package dsdbench

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/stevvooe/continuity"
)

const Scratch = layer.ChainID("")

type LayerInit func(root string) error

func CreateLayer(ls layer.Store, parent layer.ChainID, layerFunc LayerInit) (l layer.Layer, err error) {
	containerID := stringid.GenerateRandomID()
	mount, err := ls.CreateRWLayer(containerID, parent, "", nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create rw layer")
	}

	defer func() {
		if _, err1 := ls.ReleaseRWLayer(mount); err == nil {
			err = err1
		}
	}()

	path, err := mount.Mount("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to mount")
	}

	defer func() {
		if err1 := mount.Unmount(); err == nil {
			err = err1
		}
	}()

	if err := layerFunc(path); err != nil {
		return nil, errors.Wrap(err, "failed to initalize layer")
	}

	ts, err := mount.TarStream()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tar stream")
	}

	l, err = ls.Register(ts, parent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to registry new layer")
	}

	if err := ts.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close tar stream")
	}

	return l, nil
}

func CreateLayerChain(ls layer.Store, layerFuncs ...LayerInit) (l layer.Layer, err error) {
	var parentID layer.ChainID
	for i, lf := range layerFuncs {
		previous := l
		l, err = CreateLayer(ls, parentID, lf)
		if err != nil {
			l = nil
			err = errors.Wrapf(err, "failed to create layer %d", i+1)
			return
		}
		parentID = l.ChainID()
		if previous != nil {
			if _, err = ls.Release(previous); err != nil {
				l = nil
				err = errors.Wrapf(err, "layer %d release error", i)
				return
			}
		}
	}
	return

}

type FileApplier interface {
	ApplyFile(root string) error
}

type testFile struct {
	name       string
	content    []byte
	permission os.FileMode
}

func NewTestFile(name string, content []byte, perm os.FileMode) FileApplier {
	return &testFile{
		name:       name,
		content:    content,
		permission: perm,
	}
}

func (tf *testFile) ApplyFile(root string) error {
	fullPath := filepath.Join(root, tf.name)
	if err := ioutil.WriteFile(fullPath, tf.content, tf.permission); err != nil {
		return err
	}

	if err := os.Chmod(fullPath, tf.permission); err != nil {
		return err
	}

	return nil
}

type removeFile struct {
	name string
}

func RemoveFile(name string) FileApplier {
	return &removeFile{
		name: name,
	}
}

func (rf *removeFile) ApplyFile(root string) error {
	return os.RemoveAll(filepath.Join(root, rf.name))
}

type directoryCreate struct {
	name       string
	permission os.FileMode
}

func CreateDirectory(name string, perm os.FileMode) FileApplier {
	return &directoryCreate{
		name:       name,
		permission: perm,
	}
}

func (dc *directoryCreate) ApplyFile(root string) error {
	fullPath := filepath.Join(root, dc.name)
	if err := os.MkdirAll(fullPath, dc.permission); err != nil {
		return err
	}
	return nil
}

type rename struct {
	old string
	new string
}

func Rename(old, new string) FileApplier {
	return &rename{
		old: old,
		new: new,
	}
}

func (r *rename) ApplyFile(root string) error {
	return os.Rename(filepath.Join(root, r.old), filepath.Join(root, r.new))
}

func InitWithFiles(files ...FileApplier) LayerInit {
	return func(root string) error {
		for _, f := range files {
			if err := f.ApplyFile(root); err != nil {
				return err
			}
		}
		return nil
	}
}

func CreateMetadata(layers ...layer.Layer) ([]layer.Metadata, error) {
	metadata := make([]layer.Metadata, len(layers))
	for i := range layers {
		size, err := layers[i].Size()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get size")
		}

		diffsize, err := layers[i].DiffSize()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get diff size")
		}

		metadata[i].ChainID = layers[i].ChainID()
		metadata[i].DiffID = layers[i].DiffID()
		metadata[i].Size = size
		metadata[i].DiffSize = diffsize
	}

	return metadata, nil
}

func CheckMetadata(metadata, expectedMetadata []layer.Metadata) error {
	if len(metadata) != len(expectedMetadata) {
		return errors.Errorf("unexpected number of deletes %d, expected %d", len(metadata), len(expectedMetadata))
	}

	for i := range metadata {
		if metadata[i] != expectedMetadata[i] {
			return errors.Errorf("unexpected metadata %#v, expected: %#v", metadata[i], expectedMetadata[i])
		}
	}
	return nil
}

func releaseAndCheckDeleted(ls layer.Store, layer layer.Layer, removed ...layer.Layer) error {
	expectedMetadata, err := CreateMetadata(removed...)
	if err != nil {
		return errors.Wrap(err, "failed to create metadata")
	}
	metadata, err := ls.Release(layer)
	if err != nil {
		return errors.Wrap(err, "failed to release layer")
	}

	if err := CheckMetadata(metadata, expectedMetadata); err != nil {
		return errors.Wrap(err, "metadata check failed")
	}

	return nil
}

func CheckSameLayer(l1, l2 layer.Layer) error {
	if l1.ChainID() != l2.ChainID() {
		return errors.Errorf("mismatched ID: %s vs %s", l1.ChainID(), l2.ChainID())
	}
	if l1.DiffID() != l2.DiffID() {
		return errors.Errorf("mismatched DiffID: %s vs %s", l1.DiffID(), l2.DiffID())
	}

	size1, err := l1.Size()
	if err != nil {
		return errors.Wrap(err, "failed to get layer size")
	}

	size2, err := l2.Size()
	if err != nil {
		return errors.Wrap(err, "failed to get layer size")
	}

	if size1 != size2 {
		return errors.Errorf("mismatched size: %d vs %d", size1, size2)
	}

	p1 := l1.Parent()
	p2 := l2.Parent()
	if p1 != nil && p2 != nil {
		return CheckSameLayer(p1, p2)
	} else if p1 != nil || p2 != nil {
		return errors.Errorf("mismatched parents: %v vs %v", p1, p2)
	}

	return nil
}

func CheckLayerDiff(expected []byte, layer layer.Layer) error {
	expectedDigest := digest.FromBytes(expected)

	if digest.Digest(layer.DiffID()) != expectedDigest {
		return errors.Errorf("mismatched diff id for %s, got %s, expected %s", layer.ChainID(), layer.DiffID(), expected)
	}

	ts, err := layer.TarStream()
	if err != nil {
		return errors.Wrap(err, "failed to get tar stream")
	}
	defer ts.Close()

	actual, err := ioutil.ReadAll(ts)
	if err != nil {
		return errors.Wrap(err, "failed to read all tar stream")
	}

	if len(actual) != len(expected) {
		return errors.Errorf("mismatched tar stream size for %s, got %d, expected %d, %s", layer.ChainID(), len(actual), len(expected), byteDiffMessage(actual, expected))
	}

	actualDigest := digest.FromBytes(actual)

	if actualDigest != expectedDigest {
		return errors.Errorf("wrong digest of tar stream, got %s, expected %s, %s", actualDigest, expectedDigest, byteDiffMessage(actual, expected))
	}

	return nil
}

const maxByteLog = 4 * 1024

func byteDiffMessage(actual, expected []byte) string {
	d1, d2 := byteDiff(actual, expected)
	if len(d1) == 0 && len(d2) == 0 {
		return ""
	}

	prefix := len(actual) - len(d1)
	if len(d1) > maxByteLog || len(d2) > maxByteLog {
		return fmt.Sprintf("byte diff after %d matching bytes", prefix)
	}

	return fmt.Sprintf("byte diff after %d matching bytes %x, expected %x", prefix, d1, d2)
}

// byteDiff returns the differing bytes after the matching prefix
func byteDiff(b1, b2 []byte) ([]byte, []byte) {
	i := 0
	for i < len(b1) && i < len(b2) {
		if b1[i] != b2[i] {
			break
		}
		i++
	}

	return b1[i:], b2[i:]
}

func TarFromFiles(files ...FileApplier) ([]byte, error) {
	td, err := ioutil.TempDir("", "tar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(td)

	for _, f := range files {
		if err := f.ApplyFile(td); err != nil {
			return nil, err
		}
	}

	r, err := archive.Tar(td, archive.Uncompressed)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func CheckLayer(ls layer.Store, layerID layer.ChainID, layerFuncs ...LayerInit) error {
	td, err := ioutil.TempDir("", "check-layer")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(td)

	for _, lf := range layerFuncs {
		if err := lf(td); err != nil {
			return errors.Wrap(err, "failed to initialize expected layer")
		}
	}

	containerID := stringid.GenerateRandomID()
	rw, err := ls.CreateRWLayer(containerID, layerID, "", nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create rw layer")
	}

	testDir, err := rw.Mount("")
	if err != nil {
		return errors.Wrap(err, "failed to mount")
	}

	testErr := CheckDirectoryEqual(testDir, td)

	if err := rw.Unmount(); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	if _, err := ls.ReleaseRWLayer(rw); err != nil {
		return errors.Wrap(err, "failed to release rw layer")
	}

	return testErr
}

func CheckDirectoryEqual(d1, d2 string) error {
	c1, err := continuity.NewContext(d1)
	if err != nil {
		return errors.Wrap(err, "failed to build context")
	}

	c2, err := continuity.NewContext(d2)
	if err != nil {
		return errors.Wrap(err, "failed to build context")
	}

	m1, err := continuity.BuildManifest(c1)
	if err != nil {
		return errors.Wrap(err, "failed to build manifest")
	}

	m2, err := continuity.BuildManifest(c2)
	if err != nil {
		return errors.Wrap(err, "failed to build manifest")
	}

	diff := diffResourceList(m1.Resources, m2.Resources)
	if diff.HasDiff() {
		return errors.Errorf("directory diff between %s and %s\n%s", d1, d2, diff.String())
	}

	return nil
}
