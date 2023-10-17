package util

import (
	"bytes"
	"compress/gzip"
	"io"
)

func GzipData(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write(input)
	if err != nil {
		return nil, err
	}

	err = gw.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func GunzipData(input []byte) ([]byte, error) {
	r := bytes.NewReader(input)
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	decompressedData, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}
