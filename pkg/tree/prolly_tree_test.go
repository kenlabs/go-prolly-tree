package tree

import (
	"bytes"
	"context"
	"github.com/ipld/go-ipld-prime"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/zeebo/assert"
	"math/rand"
	"testing"
)

func TestProllyTreeRoundTrip(t *testing.T) {
	ctx := context.Background()
	ns := TestMemNodeStore()
	bns := ns.(*BlockNodeStore)

	cfg := DefaultChunkConfig()
	framwork, err := NewFramework(ctx, bns, cfg, nil)
	assert.NoError(t, err)

	testKeys, testVals := RandomTestData(100000)
	err = framwork.AppendBatch(ctx, testKeys, testVals)
	assert.NoError(t, err)
	tree, treeCid, err := framwork.BuildTree(ctx)
	assert.NoError(t, err)
	oldTreeCid := treeCid

	firstKey, err := tree.FirstKey()
	assert.NoError(t, err)
	assert.Equal(t, testKeys[0], firstKey)
	lastKey, err := tree.LastKey()
	assert.NoError(t, err)
	assert.Equal(t, testKeys[len(testKeys)-1], lastKey)
	assert.Equal(t, tree.TreeCount(), uint32(100000))

	firstProof, err := tree.GetProof(firstKey)
	assert.NoError(t, err)
	assert.Equal(t, firstProof[len(firstProof)-1].Node, treeCid)

	for i := 0; i < 100000; i++ {
		idx := rand.Intn(100000)
		val, err := tree.Get(testKeys[idx])
		assert.NoError(t, err)
		vnode, _ := val.AsBytes()
		trueVnode, _ := testVals[idx].AsBytes()
		assert.Equal(t, vnode, trueVnode)
	}

	newValNode := basicnode.NewBytes([]byte("test new values！!aAbB"))
	err = tree.Mutate()
	assert.NoError(t, err)

	err = tree.Put(ctx, testKeys[19999], newValNode)
	assert.NoError(t, err)

	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)

	assert.Equal(t, tree.TreeCount(), uint32(100000))

	reloadTree, err := LoadProllyTreeFromRootCid(oldTreeCid, bns)
	assert.NoError(t, err)
	for i := 0; i < 100000; i++ {
		idx := rand.Intn(100000)
		val, err := reloadTree.Get(testKeys[idx])
		assert.NoError(t, err)
		vnode, _ := val.AsBytes()
		trueVnode, _ := testVals[idx].AsBytes()
		assert.Equal(t, vnode, trueVnode)
	}

	// old
	reloadOldTree, err := LoadProllyTreeFromRootCid(oldTreeCid, bns)
	assert.NoError(t, err)
	val, err := reloadOldTree.Get(testKeys[19999])
	assert.NoError(t, err)
	vnode, _ := val.AsBytes()
	trueVnode, _ := testVals[19999].AsBytes()
	assert.Equal(t, vnode, trueVnode)

	// Make sure proofs work after relload
	reloadProof, err := reloadOldTree.GetProof(testKeys[19999])
	assert.NoError(t, err)
	assert.Equal(t, reloadProof[len(reloadProof)-1].Node, oldTreeCid)

	// insert
	newTestKey := []byte("testkey123321")
	insertVnode := basicnode.NewString("dasdsadasdsad")
	err = tree.Mutate()
	assert.NoError(t, err)
	err = tree.Put(ctx, newTestKey, insertVnode)
	assert.Equal(t, vnode, trueVnode)
	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)
	assert.Equal(t, tree.TreeCount(), uint32(100001))

	res, err := tree.Get(newTestKey)
	assert.NoError(t, err)
	expectStr, _ := insertVnode.AsString()
	resStr, _ := res.AsString()
	assert.Equal(t, expectStr, resStr)

}

func TestProllyTreeMutate(t *testing.T) {
	ctx := context.Background()
	testKeys, testVals := RandomTestData(10000)
	testAddKeys, testAddVals := RandomTestData(10000)

	tree, _ := BuildTestTreeFromData(t, testKeys, testVals)
	err := tree.Mutate()
	assert.NoError(t, err)
	for i := 0; i < len(testAddKeys); i++ {
		err = tree.Put(ctx, testAddKeys[i], testAddVals[i])
		assert.NoError(t, err)
	}

	for i := len(testKeys) / 2; i < len(testKeys); i++ {
		err = tree.Delete(ctx, testKeys[i])
		assert.NoError(t, err)
	}

	var motifiedVal []ipld.Node
	for i := len(testKeys) / 3; i < (len(testKeys)*3)/5; i++ {
		val := make([]byte, (testRand.Int63()%30)+15)
		testRand.Read(val)
		valNd := basicnode.NewBytes(val)
		motifiedVal = append(motifiedVal, valNd)
		err = tree.Put(ctx, testKeys[i], valNd)
	}

	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)

	for i := 0; i < len(testAddKeys); i++ {
		val, err := tree.Get(testAddKeys[i])
		if err != nil {
			t.Log(i)
		}
		assert.NoError(t, err)
		valBytes, err := val.AsBytes()
		assert.NoError(t, err)
		trueBytes, err := testAddVals[i].AsBytes()
		assert.NoError(t, err)
		assert.Equal(t, valBytes, trueBytes)
	}

	for i := 0; i < len(testKeys); i++ {
		val, err := tree.Get(testKeys[i])
		if i >= len(testKeys)/3 && i < (len(testKeys)*3)/5 {
			assert.NoError(t, err)
			valBytes, err := val.AsBytes()
			assert.NoError(t, err)
			trueBytes, err := motifiedVal[i-len(testKeys)/3].AsBytes()
			assert.NoError(t, err)
			assert.Equal(t, valBytes, trueBytes)
		} else if i >= (len(testKeys)*3)/5 {
			assert.Equal(t, err, KeyNotFound)
			assert.Nil(t, val)
		} else {
			assert.NoError(t, err)
			valBytes, err := val.AsBytes()
			assert.NoError(t, err)
			trueBytes, err := testVals[i].AsBytes()
			assert.NoError(t, err)
			assert.Equal(t, valBytes, trueBytes)
		}
	}
}

func TestRepeatedMutate(t *testing.T) {
	for i := 0; i < 10; i++ {
		TestProllyTreeMutate(t)
	}
}

func TestMutateEmpty(t *testing.T) {
	ctx := context.Background()
	testKeys, testVals := RandomTestData(10000)

	ns := TestMemNodeStore()
	cfg := DefaultChunkConfig()
	cfg.Strategy.Suffix.ChunkingFactor = 10
	framework, err := NewFramework(ctx, ns, cfg, nil)
	assert.NoError(t, err)
	tree, _, _ := framework.BuildTree(ctx)

	err = tree.Mutate()
	assert.NoError(t, err)

	for i := 5000; i < 10000; i++ {
		err = tree.Put(ctx, testKeys[i], testVals[i])
		assert.NoError(t, err)
	}

	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)

	for i := 5000; i < 10000; i++ {
		v, err := tree.Get(testKeys[i])
		assert.NoError(t, err)
		vBytes, err := v.AsBytes()
		assert.NoError(t, err)
		tvBytes, err := testVals[i].AsBytes()
		assert.NoError(t, err)
		assert.Equal(t, vBytes, tvBytes)
	}
}

func TestMutateSearch(t *testing.T) {
	ctx := context.Background()
	testKeys, testVals := RandomTestData(10000)

	tree, _ := BuildTestTreeFromData(t, testKeys[:5000], testVals[:5000])
	err := tree.Mutate()
	assert.NoError(t, err)

	for i := 5000; i < 10000; i++ {
		err = tree.Put(ctx, testKeys[i], testVals[i])
		assert.NoError(t, err)
	}

	for i := 0; i < 10000; i++ {
		v, err := tree.Get(testKeys[i])
		assert.NoError(t, err)
		vBytes, err := v.AsBytes()
		assert.NoError(t, err)
		tvBytes, err := testVals[i].AsBytes()
		assert.NoError(t, err)
		assert.Equal(t, vBytes, tvBytes)
	}
}

func TestPrefixCompare(t *testing.T) {
	prefixA := []byte("key1")
	prefixB := []byte("key1a")
	prefixC := []byte("key1bsada")
	prefixD := []byte("key2asada")

	t.Log(DefaultCompareFunc(prefixA, prefixB))
	t.Log(DefaultCompareFunc(prefixA, prefixC))
	t.Log(DefaultCompareFunc(prefixB, prefixC))
	t.Log(DefaultCompareFunc(prefixD, prefixC))
	t.Log(bytes.HasPrefix(prefixC, prefixB))
	t.Log(bytes.HasPrefix(prefixC, prefixA))
}

func TestCriticalCondition(t *testing.T) {
	ctx := context.Background()
	ns := TestMemNodeStore()
	bns := ns.(*BlockNodeStore)

	cfg := DefaultChunkConfig()
	framwork, err := NewFramework(ctx, bns, cfg, nil)
	assert.NoError(t, err)
	err = framwork.Append(ctx, []byte{byte(0), byte(0)}, basicnode.NewString("abcdsad"))
	assert.NoError(t, err)
	err = framwork.Append(ctx, []byte{byte(0), byte(1)}, basicnode.NewString("abcdsad"))
	assert.NoError(t, err)
	err = framwork.Append(ctx, []byte{byte(0), byte(1), byte(12)}, basicnode.NewString("abcdsad"))
	assert.NoError(t, err)
	tree, _, err := framwork.BuildTree(ctx)
	assert.NoError(t, err)
	t.Log(tree.root.Keys)

	err = tree.Mutate()
	assert.NoError(t, err)
	err = tree.Put(ctx, []byte{byte(0), byte(117), byte(115), byte(101)}, basicnode.NewString("czxcas"))
	assert.NoError(t, err)
	err = tree.Put(ctx, []byte{byte(0), byte(117), byte(115), byte(102)}, basicnode.NewString("cdasdas"))
	assert.NoError(t, err)
	err = tree.Put(ctx, []byte{byte(0), byte(117), byte(115), byte(103)}, basicnode.NewString("fdsfds"))
	assert.NoError(t, err)
	err = tree.Put(ctx, []byte{byte(0), byte(117), byte(115), byte(104)}, basicnode.NewString("dasdadass"))
	assert.NoError(t, err)

	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)

	t.Log(tree.root.Keys)
	start := []byte{byte(0), byte(117), byte(115), byte(100)}
	end := []byte{byte(0), byte(117), byte(115), byte(105)}
	iter, err := tree.Search(ctx, start, end)
	assert.NoError(t, err)

	for !iter.Done() {
		k, _, err := iter.Next()
		assert.NoError(t, err)
		kv, _ := k.AsString()
		t.Log([]byte(kv))
	}
}
