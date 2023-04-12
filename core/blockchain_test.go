package core

// func newBlockchainWithGenesis(t *testing.T) *Blockchain {
// 	bc, err := NewBlockChain(randomBlock(0))
// 	assert.Nil(t, err)
// 	return bc
// }

// func getPrevBlockHash(t *testing.T, bc *Blockchain, height uint32) types.Hash {
// 	prevHeader, err := bc.GetHeader(height - 1)
// 	assert.Nil(t, err)
// 	return BlockHasher{}.Hash(prevHeader)

// }

// func TestNewBlockchain(t *testing.T) {
// 	bc := newBlockchainWithGenesis(t)

// 	assert.NotNil(t, bc.validator)
// 	assert.Equal(t, bc.Height(), uint32(0))
// }

// func TestHasBlock(t *testing.T) {
// 	bc := newBlockchainWithGenesis(t)
// 	assert.True(t, bc.HasBlock(0))
// 	assert.False(t, bc.HasBlock(10))
// 	assert.False(t, bc.HasBlock(100))
// }

// func TestAddBlock(t *testing.T) {
// 	bc := newBlockchainWithGenesis(t)

// 	lenBlocks := 1000
// 	for i := 0; i < lenBlocks; i++ {

// 		block := randomBlock(uint32(i + 1))
// 		assert.Nil(t, bc.AddBlock(block))
// 	}
// 	assert.Equal(t, bc.Height(), uint32(lenBlocks))
// 	assert.Equal(t, len(bc.headers), lenBlocks+1)
// }
