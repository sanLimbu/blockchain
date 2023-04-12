package network

import (
	"fmt"
	"marvincrypto/core"
	"marvincrypto/crypto"
	"time"

	"github.com/sirupsen/logrus"
)

var defaultBlockTime = 5 * time.Second

type ServerOpts struct {
	RPCHandler RPCHandler
	Transports []Transport
	BlockTime  time.Duration
	PrivateKey *crypto.PrivateKey
}

type Server struct {
	ServerOpts
	blockTime   time.Duration
	memPool     *TxPool
	isValidator bool
	rpcCh       chan RPC
	quitCh      chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	if opts.BlockTime == time.Duration(0) {
		opts.BlockTime = defaultBlockTime
	}
	s := &Server{
		isValidator: opts.PrivateKey != nil,
		ServerOpts:  opts,
		blockTime:   opts.BlockTime,
		memPool:     NewTxPool(),
		rpcCh:       make(chan RPC),
	}

	// if opts.RPCHandler == nil {
	// 	opts.RPCHandler = NewDefaultPRCHanler(s)
	// }
	return s
}

func (s *Server) Start() {
	s.initTransports()
	ticker := time.NewTicker(s.blockTime)
free:
	for {
		select {
		case rpc := <-s.rpcCh:
			if err := s.RPCHandler.HandlerRPC(rpc); err != nil {
				logrus.Error(err)
			}
		case <-s.quitCh:
			break free
		case <-ticker.C:
			if s.isValidator {
				s.createNewBlock()
			}
		}
	}
}

func (s *Server) createNewBlock() error {
	fmt.Println("creating a new block")
	return nil
}

func (s *Server) ProcessTrasaction(from NetAddr, tx *core.Transaction) error {

	hash := tx.Hash(core.TxHasher{})
	if s.memPool.Has(hash) {
		logrus.WithFields(logrus.Fields{
			"hash": hash,
		}).Info("transaction already in meempool")
		return nil
	}

	if err := tx.Verify(); err != nil {
		return err
	}

	tx.SetFirstSeen(time.Now().UnixNano())

	logrus.WithFields(logrus.Fields{
		"hash":           hash,
		"mempool length": s.memPool.Len(),
	}).Info("adding new tx to the meempool")
	return s.memPool.Add(tx)
}

func (s *Server) initTransports() {
	for _, tr := range s.Transports {
		go func(tr Transport) {
			for rpc := range tr.Consume() {
				s.rpcCh <- rpc
			}
		}(tr)
	}

}
