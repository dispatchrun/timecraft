package timemachine

import "hash/crc32"

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

func checksum(b []byte) uint32 {
	return crc32.Checksum(b, castagnoli)
}
