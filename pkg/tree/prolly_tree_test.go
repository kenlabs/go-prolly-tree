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

	firstKey, err := tree.firstKey()
	assert.NoError(t, err)
	assert.Equal(t, testKeys[0], firstKey)
	lastKey, err := tree.lastKey()
	assert.NoError(t, err)
	assert.Equal(t, testKeys[len(testKeys)-1], lastKey)

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

	// insert
	insertVnode := basicnode.NewString("dasdsadasdsad")
	err = tree.Mutate()
	assert.NoError(t, err)
	err = tree.Put(ctx, []byte("testkey123321"), insertVnode)
	assert.Equal(t, vnode, trueVnode)
	_, err = tree.Rebuild(ctx)
	assert.NoError(t, err)

	res, err := tree.Get([]byte("testkey123321"))
	assert.NoError(t, err)
	expectStr, _ := insertVnode.AsString()
	resStr, _ := res.AsString()
	assert.Equal(t, expectStr, resStr)
}

func TestProllyTreeMutate(t *testing.T) {
	ctx := context.Background()
	testKeys, testVals := RandomTestData(10000)
	testAddKeys, testAddVals := RandomTestData(10000)

	tree := BuildTestTreeFromData(t, testKeys, testVals)
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
