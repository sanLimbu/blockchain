package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyPairSignVerifySuccess(t *testing.T) {
	privateKey := GeneratePrivateKey()
	publicKey := privateKey.PublicKey()
	// address := publicKey.Address()
	// fmt.Println(address)

	msg := []byte("marvin crypto")
	sig, err := privateKey.Sign(msg)
	assert.Nil(t, err)
	//fmt.Println(sig)

	assert.True(t, sig.Verify(publicKey, msg))
}

func TestKeyPairSignVerifyFail(t *testing.T) {
	privateKey := GeneratePrivateKey()

	// address := publicKey.Address()
	// fmt.Println(address)

	msg := []byte("marvin crypto")
	sig, err := privateKey.Sign(msg)
	assert.Nil(t, err)
	//fmt.Println(sig)

	otherPrivateKey := GeneratePrivateKey()
	publicKey := otherPrivateKey.PublicKey()

	assert.False(t, sig.Verify(publicKey, msg))
	assert.False(t, sig.Verify(publicKey, []byte("hiii crypto9")))
}
