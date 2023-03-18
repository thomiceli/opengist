package git

import (
	"bytes"
	"io"
)

func truncateCommandOutput(out io.Reader, maxBytes int64) (string, bool, error) {
	var (
		buf []byte
		err error
	)

	if maxBytes < 0 {
		// read entire output
		buf, err = io.ReadAll(out)
		if err != nil {
			return "", false, err
		}
		return string(buf), false, nil
	}

	// read up to maxBytes bytes
	buf = make([]byte, maxBytes)
	n, err := io.ReadFull(out, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", false, err
	}
	bytesRead := int64(n)

	// find index of last newline character
	lastNewline := bytes.LastIndexByte(buf, '\n')
	if lastNewline >= 0 {
		// truncate buffer to exclude last line
		buf = buf[:lastNewline]
	}

	return string(buf), bytesRead == maxBytes, nil
}
