package main

import (
	"bytes"
	"fmt"
	"log"
	"marvincrypto/core"
	"marvincrypto/crypto"
	"marvincrypto/network"
	"math/rand"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {

	trLocal := network.NewLocalTransport("LOCAL")
	trRemoteA := network.NewLocalTransport("REMOTE_A")
	trRemotB := network.NewLocalTransport("REMOTE_B")
	trRemoteC := network.NewLocalTransport("REMOTE_C")

	trLocal.Connect(trRemoteA)
	trRemoteA.Connect(trRemotB)
	trRemotB.Connect(trRemoteC)
	trRemoteA.Connect(trLocal)

	initRemoteServer([]network.Transport{trRemoteA, trRemotB, trRemoteC})

	go func() {
		for {
			if err := sendTransaction(trRemoteA, trLocal.Addr()); err != nil {
				logrus.Error(err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		time.Sleep(8 * time.Second)
		trLate := network.NewLocalTransport("LATE_REMOTE")
		trRemoteC.Connect(trLate)
		lateServer := makeServer(string(trLate.Addr()), trLate, nil)

		go lateServer.Start()
	}()

	privateKey := crypto.GeneratePrivateKey()
	localServer := makeServer("LOCAL", trLocal, &privateKey)
	localServer.Start()

}

func initRemoteServer(trs []network.Transport) {
	for i := 0; i < len(trs); i++ {
		id := fmt.Sprintf("REMOTE_%d", i)
		s := makeServer(id, trs[i], nil)
		go s.Start()
	}
}

func makeServer(id string, tr network.Transport, pk *crypto.PrivateKey) *network.Server {
	opts := network.ServerOpts{
		PrivateKey: pk,
		ID:         id,
		Transports: []network.Transport{tr},
	}
	s, err := network.NewServer(opts)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func sendTransaction(tr network.Transport, to network.NetAddr) error {
	privateKey := crypto.GeneratePrivateKey()
	data := []byte(strconv.FormatInt(int64(rand.Intn(1000)), 10))
	tx := core.NewTransaction(data)
	tx.Sign(privateKey)
	buf := &bytes.Buffer{}
	if err := tx.Encode(core.NewGobTxEncoder(buf)); err != nil {
		return err
	}

	msg := network.NewMessage(network.MessageTypeTx, buf.Bytes())
	return tr.SendMessage(to, msg.Bytes())

}
