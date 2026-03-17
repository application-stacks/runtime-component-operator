package utils

import (
	"bytes"
	"encoding/hex"
	"io"
	"runtime"
	"sort"

	"github.com/application-stacks/runtime-component-operator/common"
	"lukechampine.com/blake3"
)

func HashLockedData(data common.SecretMap) string {
	return hash(data, serializeLockedData)
}

func HashData(data map[string][]byte) string {
	return hash(data, serializeSecretData)
}

func hash(data any, serializer func(any) []byte) string {
	hasher := blake3.New(32, nil)
	secretBytes := serializer(data)
	io.Copy(hasher, bytes.NewReader(secretBytes))
	// clear secret bytes
	for i := range secretBytes {
		secretBytes[i] = 0
	}
	hash := hasher.Sum(nil)
	// force gc
	hasher = nil
	runtime.GC()
	return hex.EncodeToString(hash)
}

func serializeLockedData(data any) []byte {
	if _, ok := data.(common.SecretMap); !ok {
		return []byte{}
	}
	dataObj := data.(common.SecretMap)
	// sort data keys
	dataKeys := []string{}
	for k := range dataObj {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)
	// load dataBuffer delimited by a null character for every key-value pair <key>\0<value>\0
	dataBuffer := []byte{}
	for _, k := range dataKeys {
		dataBuffer = append(dataBuffer, []byte(k)...)
		dataBuffer = append(dataBuffer, '\000')
		dataBuffer = append(dataBuffer, dataObj[k].Bytes()...)
		dataBuffer = append(dataBuffer, '\000')
	}
	return dataBuffer
}

func serializeSecretData(data any) []byte {
	if _, ok := data.(map[string][]byte); !ok {
		return []byte{}
	}
	dataObj := data.(map[string][]byte)
	// sort data keys
	dataKeys := []string{}
	for k := range dataObj {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)
	// load dataBuffer delimited by a null character for every key-value pair <key>\0<value>\0
	dataBuffer := []byte{}
	for _, k := range dataKeys {
		dataBuffer = append(dataBuffer, []byte(k)...)
		dataBuffer = append(dataBuffer, '\000')
		dataBuffer = append(dataBuffer, dataObj[k]...)
		dataBuffer = append(dataBuffer, '\000')
	}
	return dataBuffer
}
