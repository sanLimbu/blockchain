package crypto

import (
	"fmt"
	"testing"
)

func TestGeneratePrivateKey(t *testing.T) {
	privateKey := GeneratePrivateKey()
	publicKey := privateKey.PublicKey()
	address := publicKey.Address()
	fmt.Println(address)
}
