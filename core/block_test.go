package core

import (
	"fmt"
	"marvincrypto/crypto"
	"marvincrypto/types"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func randomBlock(height uint32) *Block {
	header := &Header{
		Version:       1,
		PrevBlockHash: types.RandomHash(),
		Height:        height,
		Timestamp:     time.Now().UnixNano(),
	}
	tx := Transaction{
		Data: []byte("crypto"),
	}
	return NewBlock(header, []Transaction{tx})
}

func TestHasBlock(t *testing.T) {
	b := randomBlock(0)
	fmt.Println(b.Hash(BlockHasher{}))
}

func TestSignBlock(t *testing.T) {
	privateKey := crypto.GeneratePrivateKey()
	b := randomBlock(1)
	fmt.Println(b.Sign(privateKey))
	assert.Nil(t, b.Sign(privateKey))
	assert.NotNil(t, b.Signature)
}

func TestVerifyBlock(t *testing.T) {
	privateKey := crypto.GeneratePrivateKey()
	b := randomBlock(1)
	fmt.Println(b.Sign(privateKey))
	assert.Nil(t, b.Sign(privateKey))
	assert.Nil(t, b.Verify())
}
