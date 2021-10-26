package legs

import (
	"context"
	golegs "github.com/filecoin-project/go-legs"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/host"
)

var log = logging.Logger("graphsync")

type LegsCore struct {
	Host       *host.Host
	DS         *dssync.MutexDatastore
	LinkSystem *ipld.LinkSystem
}

func NewLegsCore(host *host.Host, ds *dssync.MutexDatastore, linkSys *ipld.LinkSystem) (*LegsCore, error) {

	return &LegsCore{
		Host:       host,
		DS:         ds,
		LinkSystem: linkSys,
	}, nil
}

func (core *LegsCore) NewMultiSubscriber(topic string) (golegs.LegMultiSubscriber, error) {
	lms, err := golegs.NewMultiSubscriber(context.Background(), *core.Host, core.DS, *core.LinkSystem, topic)
	if err != nil {
		return nil, err
	}
	return lms, nil
}

func (core *LegsCore) NewSubscriber(topic string) (golegs.LegSubscriber, error) {
	ls, err := golegs.NewSubscriber(context.Background(), *core.Host, core.DS, *core.LinkSystem, topic)
	if err != nil {
		return nil, err
	}

	watcher, _ := ls.OnChange()
	go validateReceived(watcher, core.DS)
	return ls, nil
}

func validateReceived(watcher chan cid.Cid, ds *dssync.MutexDatastore) {
	for {
		select {
		case downstream := <-watcher:
			if _, err := ds.Get(datastore.NewKey(downstream.String())); err != nil {
				log.Error("data not in receiver store: %v", err)
			}
		}
	}
}
