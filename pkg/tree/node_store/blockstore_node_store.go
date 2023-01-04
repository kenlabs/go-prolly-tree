package nodestore

import (
	"context"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	. "go-ipld-prolly-trees/pkg/schema"
	"go-ipld-prolly-trees/pkg/tree/linksystem"
	"go-ipld-prolly-trees/pkg/tree/types"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type StoreConfig struct {
	CacheSize int
}

var _ types.NodeStore = &NodeStore{}

type NodeStore struct {
	bs    blockstore.Blockstore
	lsys  *ipld.LinkSystem
	cache *lru.Cache
}

func NewNodeStore(bs blockstore.Blockstore, cfg *StoreConfig) (*NodeStore, error) {
	lsys := linksystem.MkLinkSystem(bs)
	ns := &NodeStore{
		bs:   bs,
		lsys: &lsys,
	}
	if cfg == nil {
		cfg = &StoreConfig{}
	}
	if cfg.CacheSize != 0 {
		var err error
		ns.cache, err = lru.New(cfg.CacheSize)
		if err != nil {
			return nil, err
		}
	}
	return ns, nil
}

func (ns *NodeStore) WriteNode(ctx context.Context, nd *ProllyNode, prefix *cid.Prefix) (cid.Cid, error) {
	var linkProto cidlink.LinkPrototype
	if prefix == nil {
		// default linkproto
		linkProto = DefaultLinkProto
	} else {
		linkProto = cidlink.LinkPrototype{Prefix: *prefix}
	}
	ipldNode, err := nd.ToNode()
	if err != nil {
		return cid.Undef, err
	}
	lnk, err := ns.lsys.Store(ipld.LinkContext{Ctx: ctx}, linkProto, ipldNode)
	if err != nil {
		return cid.Undef, err
	}
	c := lnk.(cidlink.Link).Cid

	go func() {
		if ns.cache != nil {
			ns.cache.Add(c, nd)
		}
	}()

	return c, nil
}

func (ns *NodeStore) ReadNode(ctx context.Context, c cid.Cid) (*ProllyNode, error) {
	var inCache bool
	if ns.cache != nil {
		var res interface{}
		res, inCache = ns.cache.Get(c)
		if inCache {
			return res.(*ProllyNode), nil
		}
	}
	nd, err := ns.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: c}, ProllyNodePrototype.Representation())
	if err != nil {
		return nil, err
	}

	inode, err := UnwrapProllyNode(nd)
	if err != nil {
		return nil, err
	}

	return inode, nil
}

func (ns *NodeStore) WriteTreeNode(ctx context.Context, root *ProllyTreeNode, prefix *cid.Prefix) (cid.Cid, error) {
	var linkProto cidlink.LinkPrototype
	if prefix == nil {
		// default linkproto
		linkProto = DefaultLinkProto
	} else {
		linkProto = cidlink.LinkPrototype{Prefix: *prefix}
	}
	ipldNode, err := root.ToNode()
	if err != nil {
		return cid.Undef, err
	}
	lnk, err := ns.lsys.Store(ipld.LinkContext{Ctx: ctx}, linkProto, ipldNode)
	if err != nil {
		return cid.Undef, err
	}
	c := lnk.(cidlink.Link).Cid

	go func() {
		if ns.cache != nil {
			ns.cache.Add(c, root)
		}
	}()

	return c, nil
}

func (ns *NodeStore) ReadTreeNode(ctx context.Context, c cid.Cid) (*ProllyTreeNode, error) {
	var inCache bool
	if ns.cache != nil {
		var res interface{}
		res, inCache = ns.cache.Get(c)
		if inCache {
			return res.(*ProllyTreeNode), nil
		}
	}
	nd, err := ns.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: c}, ProllyTreePrototype.Representation())
	if err != nil {
		return nil, err
	}

	root, err := UnwrapProllyRoot(nd)
	if err != nil {
		return nil, err
	}

	return root, nil
}

func (ns *NodeStore) ReadTreeConfig(ctx context.Context, c cid.Cid) (*TreeConfig, error) {
	icfg, err := ns.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: c}, ChunkConfigPrototype.Representation())
	if err != nil {
		return nil, err
	}

	cfg, err := UnwrapChunkConfig(icfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (ns *NodeStore) WriteTreeConfig(ctx context.Context, cfg *TreeConfig, prefix *cid.Prefix) (cid.Cid, error) {
	var linkProto cidlink.LinkPrototype
	if prefix == nil {
		// default linkproto
		linkProto = DefaultLinkProto
	} else {
		linkProto = cidlink.LinkPrototype{Prefix: *prefix}
	}

	ipldNode, err := cfg.ToNode()
	if err != nil {
		return cid.Undef, err
	}
	lnk, err := ns.lsys.Store(ipld.LinkContext{Ctx: ctx}, linkProto, ipldNode)
	if err != nil {
		return cid.Undef, err
	}
	c := lnk.(cidlink.Link).Cid

	return c, nil
}

func (ns *NodeStore) Close() {
}

func TestMemNodeStore() types.NodeStore {
	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	ns, _ := NewNodeStore(bs, &StoreConfig{CacheSize: 1 << 14})
	return ns
}
