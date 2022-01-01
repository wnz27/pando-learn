package mock

import (
	"context"
	"fmt"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"pando/pkg/legs"
	"pando/pkg/metadata"
	"pando/pkg/option"
	"pando/pkg/policy"
	"pando/pkg/registry"
	"pando/pkg/registry/discovery"
)

type PandoMock struct {
	Opt      *option.Options
	DS       datastore.Batching
	BS       blockstore.Blockstore
	Host     host.Host
	Core     *legs.Core
	Registry *registry.Registry
	Discover discovery.Discoverer
	outMeta  chan *metadata.MetaRecord
}

func NewPandoMock() (*PandoMock, error) {
	ctx := context.Background()

	ds := datastore.NewMapDatastore()
	mds := dssync.MutexWrap(ds)
	h, err := libp2p.New(ctx)
	if err != nil {
		return nil, err
	}
	bs := blockstore.NewBlockstore(mds)

	mockDisco, err := NewMockDiscoverer(exceptID)
	if err != nil {
		return nil, err
	}

	r, err := registry.NewRegistry(&MockDiscoveryCfg, &MockAclCfg, mds, mockDisco, nil)
	if err != nil {
		return nil, err
	}

	limiter, err := policy.NewLimiter(policy.LimiterConfig{
		TotalRate:     BaseTokenRate,
		TotalBurst:    int(BaseTokenRate),
		Registry:      r,
		BaseTokenRate: BaseTokenRate,
	})
	if err != nil {
		return nil, err
	}

	outCh := make(chan *metadata.MetaRecord)
	core, err := legs.NewLegsCore(ctx, &h, mds, bs, outCh, limiter)
	if err != nil {
		return nil, err
	}
	r.SetCore(core)
	opt := option.New(nil)
	_, err = opt.Parse()
	if err != nil {
		return nil, err
	}

	return &PandoMock{
		DS:       mds,
		BS:       bs,
		Host:     h,
		Core:     core,
		Registry: r,
		Discover: mockDisco,
		outMeta:  outCh,
		Opt:      opt,
	}, nil
}

func (pando *PandoMock) GetMetaRecordCh() (chan *metadata.MetaRecord, error) {
	if pando.outMeta != nil {
		return pando.outMeta, nil
	}
	return nil, fmt.Errorf("nil channel")
}
