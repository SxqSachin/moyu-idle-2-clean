import { inflate } from "pako"

export const compressionFormatZlibBase64 = "zlib-base64"
export const compressionFormatZlibBinaryV1 = "zlib-bin-v1"
const wsCompressedFrameMagic = new Uint8Array([87, 83, 90, 49]) // WSZ1

export type CompressedEnvelope = {
  format: string
  data: string
  originalBytes?: number
  compressedBytes?: number
}

function fromBase64(base64: string): Uint8Array {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

export function parseCompressedJSON<T>(envelope: CompressedEnvelope): T {
	if (envelope.format !== compressionFormatZlibBase64) {
		throw new Error(`unsupported compression format: ${envelope.format}`)
	}
	const inflated = inflate(fromBase64(envelope.data))
	const text = new TextDecoder().decode(inflated)
	return JSON.parse(text) as T
}

export function parseWSCompressedBinaryJSON<T>(frame: ArrayBuffer | Uint8Array): T {
  const bytes = frame instanceof Uint8Array ? frame : new Uint8Array(frame)
  if (bytes.length <= wsCompressedFrameMagic.length) {
    throw new Error(`invalid ws compressed frame length: ${bytes.length}`)
  }
  for (let i = 0; i < wsCompressedFrameMagic.length; i += 1) {
    if (bytes[i] !== wsCompressedFrameMagic[i]) {
      throw new Error("unsupported ws compressed frame magic")
    }
  }
  const inflated = inflate(bytes.subarray(wsCompressedFrameMagic.length))
  const text = new TextDecoder().decode(inflated)
  return JSON.parse(text) as T
}
