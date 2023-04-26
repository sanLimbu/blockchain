package network

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"marvincrypto/core"
	"marvincrypto/crypto"
	"marvincrypto/types"
	"net"
	"os"
	"sync"
	"time"

	"marvincrypto/api"

	"github.com/go-kit/log"
)

var defaultBlockTime = 5 * time.Second

type ServerOpts struct {
	APIListenAddr string
	SeedNodes     []string
	ListenAddr    string
	TCPTransport  *TCPTransport
	ID            string
	Logger        log.Logger
	RPCDecodeFunc RPCDecodeFunc
	RPCProcessor  RPCProcessor
	Transports    []Transport
	BlockTime     time.Duration
	PrivateKey    *crypto.PrivateKey
}

type Server struct {
	TCPTransport *TCPTransport
	peerCh       chan *TCPPeer

	mu      sync.RWMutex
	peerMap map[net.Addr]*TCPPeer
	ServerOpts
	memPool     *TxPool
	chain       *core.Blockchain
	isValidator bool
	rpcCh       chan RPC
	quitCh      chan struct{}
	txChan      chan *core.Transaction
}

func NewServer(opts ServerOpts) (*Server, error) {
	if opts.BlockTime == time.Duration(0) {
		opts.BlockTime = defaultBlockTime
	}

	if opts.RPCDecodeFunc == nil {
		opts.RPCDecodeFunc = DefaultRPCDecodeFunc
	}

	if opts.Logger == nil {
		opts.Logger = log.NewLogfmtLogger(os.Stderr)
		opts.Logger = log.With(opts.Logger, "addr", opts.ID)
	}

	chain, err := core.NewBlockChain(opts.Logger, genesisBlock())
	if err != nil {
		return nil, err
	}

	// Channel being used to communicate between the JSON RPC server
	// and the node that will process this message.
	txChan := make(chan *core.Transaction)

	// Only boot up the API server if the config has a valid port number.
	if len(opts.APIListenAddr) > 0 {
		apiServerConfig := api.ServerConfig{
			Logger:     opts.Logger,
			ListenAddr: opts.APIListenAddr,
		}
		apiServer := api.NewServer(apiServerConfig, chain, txChan)
		go apiServer.Start()
		opts.Logger.Log("msg", "JSON API running in ", "port", opts.APIListenAddr)
	}

	peerCh := make(chan *TCPPeer)
	tr := NewTCPTransport(opts.APIListenAddr, peerCh)

	s := &Server{
		TCPTransport: tr,
		peerCh:       peerCh,
		peerMap:      make(map[net.Addr]*TCPPeer),
		ServerOpts:   opts,
		chain:        chain,
		memPool:      NewTxPool(1000),
		isValidator:  opts.PrivateKey != nil,
		rpcCh:        make(chan RPC),
		quitCh:       make(chan struct{}, 1),
		txChan:       txChan,
	}

	s.TCPTransport.peerCh = peerCh

	if s.RPCProcessor == nil {
		s.RPCProcessor = s
	}
	if s.isValidator {
		go s.validatorLoop()
	}
	return s, nil
}

func (s *Server) bootstrapNetwork() {
	for _, addr := range s.SeedNodes {
		fmt.Println("trying to connect to: ", addr)
		go func(addr string) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				fmt.Printf("could not connect to %+v\n", conn)
				return
			}
			s.peerCh <- &TCPPeer{
				conn: conn,
			}
		}(addr)
	}
}

func (s *Server) Start() {

	s.TCPTransport.Start()
	time.Sleep(time.Second * 1)
	s.bootstrapNetwork()
	s.Logger.Log("msg", "accepting TCP connection on", "addr", s.ListenAddr, "ID", s.ID)

free:
	for {
		select {
		case peer := <-s.peerCh:
			s.peerMap[peer.conn.RemoteAddr()] = peer
			go peer.readLoop(s.rpcCh)
			if err := s.sendGetStatusMessage(peer); err != nil {
				s.Logger.Log("err", err)
				continue
			}

			s.Logger.Log("msg", "peer added to the server", "outgoing", peer.Outgoing, "addr", peer.conn.RemoteAddr())

		case tx := <-s.txChan:
			if err := s.processTrasaction(tx); err != nil {
				s.Logger.Log("process transaction error", err)
			}

		case rpc := <-s.rpcCh:
			msg, err := s.RPCDecodeFunc(rpc)
			if err != nil {
				s.Logger.Log("RPC Error", err)
				continue
			}

			if err := s.RPCProcessor.ProcessMessage(msg); err != nil {
				if err != core.ErrBlockKnown {
					s.Logger.Log("error", err)
				}
			}

		case <-s.quitCh:
			break free
		}
	}
	s.Logger.Log("msg", "Server is shutting down")

}

func (s *Server) validatorLoop() {
	ticker := time.NewTicker(s.BlockTime)

	s.Logger.Log("msg", "Starting validator loop", "blockTime", s.BlockTime)

	for {
		fmt.Println("creating new block")

		if err := s.createNewBlock(); err != nil {
			s.Logger.Log("create block error", err)
		}

		<-ticker.C
	}
}

func (s *Server) createNewBlock() error {
	currentHeader, err := s.chain.GetHeader(s.chain.Height())
	if err != nil {
		return err
	}

	// For now we are going to use all transactions that are in the pending pool
	// Later on when we know the internal structure of our transaction
	// we will implement some kind of complexity function to determine how
	// many transactions can be included in a block.

	txx := s.memPool.Pending()
	block, err := core.NewBlockFromPrevHeader(currentHeader, txx)
	if err != nil {
		return err
	}
	if err := block.Sign(*s.PrivateKey); err != nil {
		return err
	}
	if err := s.chain.AddBlock(block); err != nil {
		return err
	}

	//TODO : pending pool of tx should only reflect on validator nodes.
	//Right now "normal nodes" does not have their pending pool cleared
	s.memPool.ClearPending()

	go s.broadcastBlock(block)

	return nil
}

func (s *Server) ProcessMessage(msg *DecodedMessage) error {
	switch t := msg.Data.(type) {
	case *core.Transaction:
		return s.processTrasaction(t)
	case *core.Block:
		return s.processBlock(t)
	case *GetStatusMessage:
		return s.processGetStatusMessage(msg.From, t)
	case *StatusMessage:
		return s.processStatusMessage(msg.From, t)
	case *GetBlocksMessage:
		return s.processGetBlocksMessage(msg.From, t)
	case *BlocksMessage:
		return s.processBlocksMessage(msg.From, t)

	}
	return nil
}

func (s *Server) processBlocksMessage(from net.Addr, data *BlocksMessage) error {
	s.Logger.Log("msg", "received BLOCKS!!!!!!!!", "from", from)

	for _, block := range data.Blocks {
		if err := s.chain.AddBlock(block); err != nil {
			s.Logger.Log("error", err.Error())
			return err
		}
	}

	return nil
}

func (s *Server) processGetBlocksMessage(from net.Addr, data *GetBlocksMessage) error {
	s.Logger.Log("msg", "received getBlocks message", "from", from)

	var (
		blocks    = []*core.Block{}
		ourHeight = s.chain.Height()
	)

	if data.To == 0 {
		for i := int(data.From); i <= int(ourHeight); i++ {
			block, err := s.chain.GetBlock(uint32(i))
			if err != nil {
				return err
			}
			blocks = append(blocks, block)

		}
	}

	blocksMsg := &BlocksMessage{
		Blocks: blocks,
	}
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(blocksMsg); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	msg := NewMessage(MessageTypeBlock, buf.Bytes())
	peer, ok := s.peerMap[from]
	if !ok {
		return fmt.Errorf("peer %s not known", peer.conn.RemoteAddr())
	}
	return peer.Send(msg.Bytes())

}

func (s *Server) processGetStatusMessage(from net.Addr, data *GetStatusMessage) error {

	s.Logger.Log("msg", "received getStatus message", "from", from)
	statusMessage := &StatusMessage{
		CurrentHeight: s.chain.Height(),
		ID:            s.ID,
	}
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(statusMessage); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	peer, ok := s.peerMap[from]
	if !ok {
		return fmt.Errorf("peer %s not known", peer.conn.RemoteAddr())
	}

	msg := NewMessage(MessageTypeStatus, buf.Bytes())
	return peer.Send(msg.Bytes())

}

func (s *Server) processStatusMessage(from net.Addr, data *StatusMessage) error {

	s.Logger.Log("msg", "received STATUS message", "from", from)
	if data.CurrentHeight <= s.chain.Height() {
		s.Logger.Log("msg", "cannot sync blockHeight to low", "ourHeight", s.chain.Height(), "theirHeight", data.CurrentHeight, "addr", from)
		return nil
	}
	go s.requestBlocksLoop(from)

	return nil
}

// TODO: Find a way to make sure we dont keep syncing when we are at the highest
// block height in the network.
func (s *Server) requestBlocksLoop(peer net.Addr) error {
	ticker := time.NewTicker(3 * time.Second)

	for {
		ourHeight := s.chain.Height()

		s.Logger.Log("msg", "requesting new blocks", "requesting height", ourHeight+1)

		// In this case we are 100% sure that the node has blocks heigher than us.
		getBlocksMessage := &GetBlocksMessage{
			From: ourHeight + 1,
			To:   0,
		}

		buf := new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(getBlocksMessage); err != nil {
			return err
		}
		s.mu.RLock()
		defer s.mu.RUnlock()

		msg := NewMessage(MessageTypeGetBlocks, buf.Bytes())
		peer, ok := s.peerMap[peer]
		if !ok {
			return fmt.Errorf("peer %s not known", peer.conn.RemoteAddr())
		}

		if err := peer.Send(msg.Bytes()); err != nil {
			s.Logger.Log("error", "failed to send to peer", "err", err, "peer", peer)
		}
		<-ticker.C
	}

}

func (s *Server) sendGetStatusMessage(peer *TCPPeer) error {

	var (
		getStatusMsg = new(GetStatusMessage)
		buf          = new(bytes.Buffer)
	)

	if err := gob.NewEncoder(buf).Encode(getStatusMsg); err != nil {
		return err
	}

	msg := NewMessage(MessageTypeGetStatus, buf.Bytes())
	return peer.Send(msg.Bytes())

}

func (s *Server) broadcast(payload []byte) error {
	for _, tr := range s.Transports {
		if err := tr.Broadcast(payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) processBlock(b *core.Block) error {

	if err := s.chain.AddBlock(b); err != nil {
		s.Logger.Log("error", err.Error())
		return err
	}

	go s.broadcastBlock(b)

	return nil
}

func (s *Server) processTrasaction(tx *core.Transaction) error {

	hash := tx.Hash(core.TxHasher{})

	if s.memPool.Contains(hash) {
		return nil
	}

	if err := tx.Verify(); err != nil {
		return err
	}

	// s.Logger.Log(
	// 	"msg", "adding new tx to mempool",
	// 	"hash", hash,
	// 	"mempoolPending", s.memPool.PendingCount(),
	// )

	//tx.SetFirstSeen(time.Now().UnixNano())

	go s.broadcastTx(tx)

	s.memPool.Add(tx)
	return nil
}

func (s *Server) broadcastBlock(b *core.Block) error {
	buf := &bytes.Buffer{}
	if err := b.Encode(core.NewGobBlockEncoder(buf)); err != nil {
		return err
	}
	msg := NewMessage(MessageTypeBlock, buf.Bytes())

	return s.broadcast(msg.Bytes())

}

func (s *Server) broadcastTx(tx *core.Transaction) error {
	buf := &bytes.Buffer{}
	if err := tx.Encode(core.NewGobTxEncoder(buf)); err != nil {
		return err
	}
	msg := NewMessage(MessageTypeTx, buf.Bytes())
	return s.broadcast(msg.Bytes())
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

func genesisBlock() *core.Block {
	header := &core.Header{
		Version:   1,
		DataHash:  types.Hash{},
		Height:    0,
		Timestamp: 00000,
	}
	b, _ := core.NewBlock(header, nil)

	coinbase := crypto.PublicKey{}
	tx := core.NewTransaction(nil)
	tx.From = coinbase
	tx.To = coinbase
	tx.Value = 10_000_000
	b.Transactions = append(b.Transactions, tx)
	privKey := crypto.GeneratePrivateKey()
	if err := b.Sign(privKey); err != nil {
		panic(err)
	}

	return b
}
