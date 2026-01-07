package printer

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func Create3MFTempFromFile(inputPath, internalPath string) (string, func(), error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return "", nil, err
	}
	defer in.Close()

	tmpFile, err := os.CreateTemp("", "bambu-*.3mf")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	zipWriter := zip.NewWriter(tmpFile)
	w, err := zipWriter.Create(filepath.ToSlash(internalPath))
	if err != nil {
		_ = zipWriter.Close()
		_ = tmpFile.Close()
		cleanup()
		return "", nil, err
	}
	if _, err := io.Copy(w, in); err != nil {
		_ = zipWriter.Close()
		_ = tmpFile.Close()
		cleanup()
		return "", nil, err
	}
	if err := zipWriter.Close(); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpFile.Name(), cleanup, nil
}
