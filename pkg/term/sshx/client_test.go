package sshx

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/Duke1616/ecmdb-plugins/pkg/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestRSAPEM 生成测试用的未加密 RSA 私钥 PEM 字节切片
func generateTestRSAPEM(t *testing.T) []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	})
}

func TestAuth_Passwd(t *testing.T) {
	gateway := term.Endpoint{
		AuthType: "passwd",
		Password: "test-secret-password",
	}

	method, err := Auth(gateway)
	require.NoError(t, err)
	assert.NotNil(t, method)
}

func TestAuth_PublicKey(t *testing.T) {
	privateKeyPEM := generateTestRSAPEM(t)

	gateway := term.Endpoint{
		AuthType:   "publickey",
		PrivateKey: privateKeyPEM,
	}

	method, err := Auth(gateway)
	require.NoError(t, err)
	assert.NotNil(t, method)
}

func TestAuth_InvalidPrivateKey(t *testing.T) {
	gateway := term.Endpoint{
		AuthType:   "publickey",
		PrivateKey: []byte("invalid-pem-data"),
	}

	method, err := Auth(gateway)
	require.Error(t, err)
	assert.Nil(t, method)
	assert.Contains(t, err.Error(), "解析私钥失败")
}

func TestAuth_UnsupportedType(t *testing.T) {
	gateway := term.Endpoint{
		AuthType: "unknown_type",
	}

	method, err := Auth(gateway)
	require.Error(t, err)
	assert.Nil(t, method)
	assert.Contains(t, err.Error(), "无可匹配认证类型")
}
