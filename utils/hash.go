package utils

import (
	"encoding/hex"
	"sort"

	"github.com/zeebo/blake3"
)

func HashData(data map[string][]byte) string {
	hasher := blake3.New()
	hasher.Write(serializeSecretData(data))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

func serializeSecretData(data map[string][]byte) []byte {
	// sort data keys
	dataKeys := []string{}
	for k := range data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)
	// load dataBuffer delimited by a null character for every key-value pair <key>\0<value>\0
	dataBuffer := []byte{}
	for _, k := range dataKeys {
		dataBuffer = append(dataBuffer, []byte(k)...)
		dataBuffer = append(dataBuffer, '\000')
		dataBuffer = append(dataBuffer, data[k]...)
		dataBuffer = append(dataBuffer, '\000')
	}
	return dataBuffer
}
