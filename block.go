package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"time"
)

// 区块的结构体
type Block struct {
	Timestamp int64
	// Data          []byte 不要Data，取而代之的是存储交易
	Transactions  []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

func NewBlock(Transactions []*Transaction, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), Transactions, prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce
	return block
}

// 生成创世区块
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{})
}

// 将调用的区块数据序列化
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		fmt.Println(err)
	}

	return result.Bytes()
}

func DeserializeBlock(d []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		fmt.Println(err)
	}

	return &block
}

// 计算区块Transactions的hash,以便被pow中的函数使用
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))
	return txHash[:]
}
