package mock

import (
	"context"
	goLegs "github.com/filecoin-project/go-legs"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipld/go-ipld-prime/linking"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"math/rand"
	"pando/pkg/legs"
	"time"
)

type ProviderMock struct {
	ID           peer.ID
	LegsProvider goLegs.LegPublisher
	lsys         *linking.LinkSystem
	DagService   format.DAGService
}

func getDagNodes() []format.Node {
	a := merkledag.NewRawNode([]byte("aaaaa" + string(rune(rand.Intn(100000)))))
	b := merkledag.NewRawNode([]byte("bbbbb" + string(rune(rand.Intn(100000)))))
	c := merkledag.NewRawNode([]byte("ccccc" + string(rune(rand.Intn(100000)))))

	nd1 := &merkledag.ProtoNode{}
	err := nd1.AddNodeLink("cat", a)
	if err != nil {
		return nil
	}

	nd2 := &merkledag.ProtoNode{}
	if err = nd2.AddNodeLink("first", nd1); err != nil {
		return nil
	}
	if err := nd2.AddNodeLink("dog", b); err != nil {
		return nil
	}

	nd3 := &merkledag.ProtoNode{}
	if err := nd3.AddNodeLink("second", nd2); err != nil {
		return nil
	}
	if err := nd3.AddNodeLink("bear", c); err != nil {
		return nil
	}

	return []format.Node{nd3, nd2, nd1, c, b, a}
}

func NewMockProvider(p *PandoMock) (*ProviderMock, error) {
	rand.Seed(time.Now().UnixNano())
	// mock provider legs
	srcHost, err := libp2p.New(context.Background())
	if err != nil {
		return nil, err
	}
	srcDatastore := dssync.MutexWrap(datastore.NewMapDatastore())
	srcBlockstore := blockstore.NewBlockstore(srcDatastore)
	srcLinkSystem := legs.MkLinkSystem(srcBlockstore)
	dags := merkledag.NewDAGService(blockservice.New(srcBlockstore, offline.Exchange(srcBlockstore)))
	legsPublisher, err := goLegs.NewPublisher(context.Background(), srcHost, srcDatastore, srcLinkSystem, "PandoPubSub")
	if err != nil {
		return nil, err
	}

	multiAddress := p.Host.Addrs()[0].String() + "/ipfs/" + p.Host.ID().String()
	peerInfo, err := peer.AddrInfoFromString(multiAddress)
	if err != nil {
		return nil, err
	}

	if err = srcHost.Connect(context.Background(), *peerInfo); err != nil {
		return nil, err
	}

	return &ProviderMock{
		ID:           srcHost.ID(),
		LegsProvider: legsPublisher,
		lsys:         &srcLinkSystem,
		DagService:   dags,
	}, nil
}

func (p *ProviderMock) SendDag() ([]cid.Cid, error) {
	cidlist := make([]cid.Cid, 0)

	// store test dag
	dagNodes := getDagNodes()
	for i := 0; i < len(dagNodes); i++ {
		err := p.DagService.Add(context.Background(), dagNodes[i])
		if err != nil {
			return nil, err
		}
		cidlist = append(cidlist, dagNodes[i].Cid())
	}

	err := p.LegsProvider.UpdateRoot(context.Background(), dagNodes[0].Cid())
	if err != nil {
		return nil, err
	}

	return cidlist, nil
}

func (p *ProviderMock) Close() error {
	return p.LegsProvider.Close()
}
