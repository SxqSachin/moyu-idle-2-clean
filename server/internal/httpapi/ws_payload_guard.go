package httpapi

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	inventoryInstancesUpdatedEventType = "inventory.instances.updated"
	wsBroadcastFrameMagic              = "\x89MSWGWS\n"
	wsBroadcastFrameVersion            = byte(1)
	wsBroadcastFrameDecodedMaxBytes    = 16 << 20
	wsBroadcastFrameHeaderBytes        = len(wsBroadcastFrameMagic) + 1 + 4
)

type inventoryRefreshRequiredPayload struct {
	RefreshRequired bool        `json:"refreshRequired"`
	Meta            wsPatchMeta `json:"meta"`
}

func prepareWSBroadcastEnvelopeForPublish(evt wsBroadcastEvent, maxPayload int64, allowBattleBatchCompression bool) (wsBroadcastEvent, []byte, bool, error) {
	body, err := json.Marshal(evt)
	if err != nil {
		return evt, nil, false, err
	}
	if maxPayload <= 0 || int64(len(body)) <= maxPayload {
		return evt, body, false, nil
	}
	if isBattleRunBatchWSEventType(evt.EventType) {
		if !allowBattleBatchCompression {
			return evt, nil, false, fmt.Errorf(
				"ws battle batch exceeds nats maximum while compression is disabled: bytes=%d max=%d",
				len(body), maxPayload,
			)
		}
		compressed, err := encodeWSBroadcastCompressedFrame(body)
		if err == nil && int64(len(compressed)) <= maxPayload {
			return evt, compressed, false, nil
		}
		if err != nil {
			return evt, nil, false, fmt.Errorf("compress oversized battle batch: %w", err)
		}
		return evt, nil, false, fmt.Errorf(
			"compressed battle batch exceeds nats maximum: raw_bytes=%d compressed_bytes=%d max=%d",
			len(body), len(compressed), maxPayload,
		)
	}
	if strings.TrimSpace(evt.EventType) != inventoryInstancesUpdatedEventType {
		return evt, nil, false, fmt.Errorf("ws event payload exceeds nats maximum: type=%s bytes=%d max=%d", evt.EventType, len(body), maxPayload)
	}

	var original struct {
		Meta wsPatchMeta `json:"meta"`
	}
	if err := json.Unmarshal(evt.Payload, &original); err != nil {
		return evt, nil, false, fmt.Errorf("decode oversized inventory event: %w", err)
	}
	compact, err := json.Marshal(inventoryRefreshRequiredPayload{
		RefreshRequired: true,
		Meta:            original.Meta,
	})
	if err != nil {
		return evt, nil, false, err
	}
	evt.Payload = compact
	body, err = json.Marshal(evt)
	if err != nil {
		return evt, nil, false, err
	}
	if int64(len(body)) > maxPayload {
		return evt, nil, false, fmt.Errorf("compacted inventory event still exceeds nats maximum: bytes=%d max=%d", len(body), maxPayload)
	}
	return evt, body, true, nil
}

func encodeWSBroadcastCompressedFrame(raw []byte) ([]byte, error) {
	if len(raw) > wsBroadcastFrameDecodedMaxBytes {
		return nil, fmt.Errorf("decoded payload exceeds hard limit: bytes=%d max=%d", len(raw), wsBroadcastFrameDecodedMaxBytes)
	}

	var frame bytes.Buffer
	frame.WriteString(wsBroadcastFrameMagic)
	frame.WriteByte(wsBroadcastFrameVersion)
	var decodedSize [4]byte
	binary.BigEndian.PutUint32(decodedSize[:], uint32(len(raw)))
	frame.Write(decodedSize[:])

	zw, err := zlib.NewWriterLevel(&frame, zlib.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return frame.Bytes(), nil
}

func decodeWSBroadcastEnvelope(body []byte) (wsBroadcastEvent, bool, error) {
	raw, compressed, err := decodeWSBroadcastFrame(body)
	if err != nil {
		return wsBroadcastEvent{}, compressed, err
	}
	var evt wsBroadcastEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		return wsBroadcastEvent{}, compressed, err
	}
	if compressed && !isBattleRunBatchWSEventType(evt.EventType) {
		return wsBroadcastEvent{}, true, fmt.Errorf("compressed ws event type is not supported: type=%s", evt.EventType)
	}
	return evt, compressed, nil
}

func decodeWSBroadcastFrame(body []byte) ([]byte, bool, error) {
	if !isWSBroadcastCompressedFrame(body) {
		return body, false, nil
	}
	if len(body) < wsBroadcastFrameHeaderBytes {
		return nil, true, fmt.Errorf("compressed ws frame header is truncated: bytes=%d", len(body))
	}
	versionOffset := len(wsBroadcastFrameMagic)
	if body[versionOffset] != wsBroadcastFrameVersion {
		return nil, true, fmt.Errorf("compressed ws frame version is unsupported: version=%d", body[versionOffset])
	}
	decodedSizeOffset := versionOffset + 1
	expectedSize := int64(binary.BigEndian.Uint32(body[decodedSizeOffset : decodedSizeOffset+4]))
	if expectedSize > wsBroadcastFrameDecodedMaxBytes {
		return nil, true, fmt.Errorf("compressed ws frame declares payload above hard limit: bytes=%d max=%d", expectedSize, wsBroadcastFrameDecodedMaxBytes)
	}

	zr, err := zlib.NewReader(bytes.NewReader(body[wsBroadcastFrameHeaderBytes:]))
	if err != nil {
		return nil, true, fmt.Errorf("open compressed ws frame: %w", err)
	}
	var decoded bytes.Buffer
	_, copyErr := io.Copy(&decoded, io.LimitReader(zr, wsBroadcastFrameDecodedMaxBytes+1))
	closeErr := zr.Close()
	if copyErr != nil {
		return nil, true, fmt.Errorf("decode compressed ws frame: %w", copyErr)
	}
	if closeErr != nil {
		return nil, true, fmt.Errorf("close compressed ws frame: %w", closeErr)
	}
	if decoded.Len() > wsBroadcastFrameDecodedMaxBytes {
		return nil, true, fmt.Errorf("decoded ws frame exceeds hard limit: bytes>%d", wsBroadcastFrameDecodedMaxBytes)
	}
	if int64(decoded.Len()) != expectedSize {
		return nil, true, fmt.Errorf("compressed ws frame size mismatch: got=%d want=%d", decoded.Len(), expectedSize)
	}
	return decoded.Bytes(), true, nil
}

func isWSBroadcastCompressedFrame(body []byte) bool {
	return len(body) >= len(wsBroadcastFrameMagic) && bytes.Equal(body[:len(wsBroadcastFrameMagic)], []byte(wsBroadcastFrameMagic))
}
