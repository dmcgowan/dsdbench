package dsdbench

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/layer"
	"github.com/pkg/errors"
)

var (
	testDirectory string
	keepTestDir   bool
)

func init() {
	flag.StringVar(&testDirectory, "dir", "", "Default root of test directory")
	flag.BoolVar(&keepTestDir, "keep", false, "Keep test file directory")
}

type layerStore struct {
	layer.Store

	tempDir string
}

func (ls *layerStore) Cleanup() error {
	if err := ls.Store.Cleanup(); err != nil {
		return errors.Wrapf(err, "cleanup error, kept %s", ls.tempDir)
	}
	if keepTestDir {
		fmt.Printf("Kept root directory: %s\n", ls.tempDir)
		return nil
	}
	return os.RemoveAll(ls.tempDir)
}

func getLayerStore() (layer.Store, error) {
	td, err := ioutil.TempDir(testDirectory, "layer-test-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	driverName := os.Getenv("DOCKER_GRAPHDRIVER")
	if driverName == "" {
		return nil, errors.New("no graphdriver specified")
	}

	var driverOptions []string
	if options := os.Getenv("DOCKER_GRAPHDRIVER_OPTIONS"); options != "" {
		driverOptions = strings.Split(options, " ")
	}

	gd, err := graphdriver.GetDriver(driverName, td, driverOptions, nil, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get graph driver")
	}
	fms, err := layer.NewFSMetadataStore(filepath.Join(td, "layer"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get metadata store")
	}

	ls, err := layer.NewStoreFromGraphDriver(fms, gd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create layer store")
	}

	return &layerStore{
		Store:   ls,
		tempDir: td,
	}, nil
}
