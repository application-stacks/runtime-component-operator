package utils

import (
	"testing"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestHashData(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	data := map[string][]byte{}
	data["xyz"] = []byte("contentforxyz")
	data["abc"] = []byte("1Ag@aZ821Sd1asd1231nkgrniekghis168adf")
	testGHFD := []Test{
		{"Serialize sample data", []byte("abc\0001Ag@aZ821Sd1asd1231nkgrniekghis168adf\000xyz\000contentforxyz\000"), serializeSecretData(data)},
		{"Get hash from serialized data", "2d0b4d0adc4124bdfb959cb8b584473b5392cf2287b69a11663b288c90cfa010", HashData(data)},
	}
	verifyTests(testGHFD, t)
}
