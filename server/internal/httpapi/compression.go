package httpapi

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

const wsCompressedFrameMagic = "WSZ1"
const wsIncomingMessageMaxBytes int64 = 64 << 10

func encodeWSCompressedFrame(plain []byte, level int) ([]byte, error) {
	compressed, err := compressRawZlib(plain, level)
	if err != nil {
		return nil, err
	}
	framed := make([]byte, len(wsCompressedFrameMagic)+len(compressed))
	copy(framed, wsCompressedFrameMagic)
	copy(framed[len(wsCompressedFrameMagic):], compressed)
	return framed, nil
}

func decodeWSCompressedFrame(frame []byte) ([]byte, error) {
	if len(frame) <= len(wsCompressedFrameMagic) {
		return nil, fmt.Errorf("invalid ws compressed frame length: %d", len(frame))
	}
	if string(frame[:len(wsCompressedFrameMagic)]) != wsCompressedFrameMagic {
		return nil, fmt.Errorf("unsupported ws compressed frame magic")
	}
	return decompressRawZlib(frame[len(wsCompressedFrameMagic):], wsIncomingMessageMaxBytes)
}

func compressRawZlib(raw []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressRawZlib(raw []byte, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid zlib output limit")
	}
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("open zlib reader failed: %w", err)
	}
	defer reader.Close()
	plain, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("inflate zlib data failed: %w", err)
	}
	if int64(len(plain)) > maxBytes {
		return nil, fmt.Errorf("inflated ws payload exceeds %d bytes", maxBytes)
	}
	return plain, nil
}
