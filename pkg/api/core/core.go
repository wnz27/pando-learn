package core

import (
	badger "github.com/dgraph-io/badger/v3"
	"github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ds-leveldb"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"

	"pando/pkg/legs"
	"pando/pkg/lotus"
	"pando/pkg/metadata"
	"pando/pkg/registry"
	"pando/pkg/statetree"
)

type Core struct {
	MetaManager   *metadata.MetaManager
	StateTree     *statetree.StateTree
	LotusDiscover *lotus.Discoverer
	Registry      *registry.Registry
	LegsCore      *legs.Core
	StoreInstance *StoreInstance
	LinkSystem    *ipld.LinkSystem
}

type StoreInstance struct {
	DataStore      *leveldb.Datastore
	MutexDataStore *sync.MutexDatastore
	BlockStore     blockstore.Blockstore
	CacheStore     *badger.DB
}
