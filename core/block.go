package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"marvincrypto/crypto"
	"marvincrypto/types"
)

type Header struct {
	Version       uint32
	DataHash      types.Hash
	PrevBlockHash types.Hash
	Timestamp     int64
	Height        uint32
	Nonce         uint64
}

func (h *Header) Bytes() []byte {
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)
	enc.Encode(h)
	return buf.Bytes()
}

type Block struct {
	*Header
	Transactions []Transaction
	Validator    crypto.PublicKey
	Signature    *crypto.Signature

	//cache the version of header hash
	hash types.Hash
}

func NewBlock(h *Header, txx []Transaction) *Block {
	return &Block{
		Header:       h,
		Transactions: txx,
	}
}

func (b *Block) Sign(privateKey crypto.PrivateKey) error {

	sig, err := privateKey.Sign(b.Header.Bytes())
	if err != nil {
		return err
	}
	b.Validator = privateKey.PublicKey()
	b.Signature = sig
	return nil

}

func (b *Block) Verify() error {
	if b.Signature == nil {
		return fmt.Errorf("block has no signature")
	}
	if !b.Signature.Verify(b.Validator, b.Header.Bytes()) {
		return fmt.Errorf("block has invalid signature")
	}

	for _, tx := range b.Transactions {
		if err := tx.Verify(); err != nil {
			return err
		}
	}
	return nil
}

func (b *Block) Decode(dec Decoder[*Block]) error {
	return dec.Decode(b)
}

func (b *Block) Encode(dec Encoder[*Block]) error {
	return dec.Encode(b)
}

func (b *Block) Hash(hasher Hasher[*Header]) types.Hash {
	if b.hash.IsZero() {
		b.hash = hasher.Hash(b.Header)
	}
	return b.hash
}
