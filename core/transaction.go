package core

import (
	"fmt"
	"marvincrypto/crypto"
	"marvincrypto/types"
)

type Transaction struct {
	Data      []byte
	PublicKey crypto.PublicKey
	Signature *crypto.Signature
	hash      types.Hash
	//firstseen is the timestamp of when this tx is first seen locally
	firstSeen int64
}

func NewTransaction(data []byte) *Transaction {
	return &Transaction{
		Data: data,
	}
}

func (tx *Transaction) Hash(hasher Hasher[*Transaction]) types.Hash {
	if tx.hash.IsZero() {
		tx.hash = hasher.Hash(tx)
	}
	return tx.hash
}

func (tx *Transaction) Sign(privateKey crypto.PrivateKey) error {
	sig, err := privateKey.Sign(tx.Data)
	if err != nil {
		return err
	}
	tx.PublicKey = privateKey.PublicKey()
	tx.Signature = sig
	return nil
}

func (tx *Transaction) Verify() error {
	if tx.Signature == nil {
		return fmt.Errorf("Transaction has no signature")
	}
	if !tx.Signature.Verify(tx.PublicKey, tx.Data) {
		return fmt.Errorf("invalid transaction signature")
	}
	return nil
}

func (tx *Transaction) SetFirstSeen(t int64) {
	tx.firstSeen = t
}

func (tx *Transaction) FirstSeen() int64 {
	return tx.firstSeen
}

func (tx *Transaction) Decode(dec Decoder[*Transaction]) error {
	return dec.Decode(tx)
}

func (tx *Transaction) Encode(enc Encoder[*Transaction]) error {
	return enc.Encode(tx)
}
