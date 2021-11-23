package legs

import (
	"Pando/internal/account"
	"bytes"
	"context"
	"fmt"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-graphsync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/time/rate"
	"io"
	"math"
	"time"

	// dagjson codec registered for encoding

	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

func MkLinkSystem(bs blockstore.Blockstore) ipld.LinkSystem {
	lsys := cidlink.DefaultLinkSystem()
	lsys.TrustedStorage = true
	lsys.StorageReadOpener = func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("unsupported link type")
		}
		block, err := bs.Get(asCidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(block.RawData()), nil
	}
	lsys.StorageWriteOpener = func(lnkCtx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		var buffer settableBuffer
		committer := func(lnk ipld.Link) error {
			asCidLink, ok := lnk.(cidlink.Link)
			if !ok {
				return fmt.Errorf("unsupported link type")
			}
			block, err := blocks.NewBlockWithCid(buffer.Bytes(), asCidLink.Cid)
			if err != nil {
				return err
			}
			return bs.Put(block)
		}
		return &buffer, committer, nil
	}
	return lsys
}

type settableBuffer struct {
	bytes.Buffer
	didSetData bool
	data       []byte
}

func (sb *settableBuffer) SetBytes(data []byte) error {
	sb.didSetData = true
	sb.data = data
	return nil
}

func (sb *settableBuffer) Bytes() []byte {
	if sb.didSetData {
		return sb.data
	}
	return sb.Buffer.Bytes()
}

// storageHook determines the logic to run when a new block is received through
// graph sync.
//
// When we receive a block, if it is not an advertisement it means that we
// finished storing the list of entries of the advertisement, so we are ready
// to process them and ingest into the indexer core.
func (l *Core) storageHook() graphsync.OnIncomingBlockHook {
	return func(p peer.ID, responseData graphsync.ResponseData, blockData graphsync.BlockData, hookActions graphsync.IncomingBlockHookActions) {
		log.Debug("hook - Triggering after a block has been stored")
		// Get cid of the node received.
		c := blockData.Link().(cidlink.Link).Cid

		// Get entries node from datastore.
		_, err := l.BS.Get(c)
		if err != nil {
			log.Errorf("Error while fetching the node from datastore: %s", err)
			return
		}

		log.Debugf("[recv] block from graphysnc.cid %s\n", c.String())
	}
}

func (l *Core) rateLimitHook() graphsync.OnOutgoingRequestHook {
	return func(p peer.ID, request graphsync.RequestData, hookActions graphsync.OutgoingRequestHookActions) {
		accountInfo := account.FetchPeerType(p, l.rateLimiter.Config().Registry)
		peerRateLimiter := l.rateLimiter.PeerLimiter(p)
		if peerRateLimiter == nil {
			peerRateLimiter = l.addPeerLimiter(p, accountInfo.PeerType, accountInfo.AccountLevel)
		}
		log.Debugf("rate limit for peer %s is %f token/s, accountLevel is %v", p, peerRateLimiter.Limit(), accountInfo.AccountLevel)
		if !l.rateLimiter.Allow() || !peerRateLimiter.Allow() {
			const limitError = "your request was canceled because of the rate limit policy"
			//go l.cancelRequest(request.ID())
			go l.pauseRequest(request.ID())
			log.Warnf(limitError)
			go l.unpauseRequest(request.ID(), peerRateLimiter)
			log.Debugf("leave rateLimitHook")
			return
		}
		log.Debugf("request %d from peer %s allowed", request.ID(), p)
	}
}

func (l *Core) cancelRequest(request graphsync.RequestID) {
	if err := l.lms.GraphSync().CancelRequest(context.Background(), request); err != nil {
		log.Warnf("cancel request failed, error: %s", err.Error())
	}
	log.Debugf("request %d canceled", request)
}

func (l *Core) pauseRequest(request graphsync.RequestID) {
	if err := l.lms.GraphSync().PauseRequest(request); err != nil {
		log.Warnf("pause request failed, error: %s", err.Error())
	}
}

func (l *Core) unpauseRequest(request graphsync.RequestID, peerRateLimiter *rate.Limiter) {
	time.Sleep(time.Millisecond)
	log.Debugf("token for request %d = %f", request, peerRateLimiter.Limit())
	if l.rateLimiter.Allow() && peerRateLimiter.Allow() {
		if err := l.lms.GraphSync().UnpauseRequest(request); err != nil {
			log.Warnf("unpause request %d failed, error: %s", request, err.Error())
		} else {
			log.Debugf("request %d unpaused", request)
		}
	} else {
		l.unpauseRequest(request, peerRateLimiter)
	}
}

func (l *Core) addPeerLimiter(peerID peer.ID, peerType account.PeerType, accountLevel int) *rate.Limiter {
	var tokenRate float64
	switch peerType {
	case account.UnregisteredPeer:
		tokenRate = 0.1 * l.rateLimiter.Config().BaseTokenRate
	case account.WhiteListPeer:
		tokenRate = 0.5 * l.rateLimiter.Config().BaseTokenRate
	case account.RegisteredPeer:
		tokenRate = l.registeredPeerTokenRate(accountLevel)
	}

	return l.rateLimiter.AddPeerLimiter(peerID, tokenRate, int(math.Floor(tokenRate)))
}

func (l *Core) registeredPeerTokenRate(accountLevel int) float64 {
	levelCount := l.rateLimiter.Config().Registry.AccountLevelCount()
	weight := float64(accountLevel / levelCount)
	return weight * 0.4 * l.rateLimiter.Config().BaseTokenRate
}
