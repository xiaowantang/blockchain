package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

// 挖出一个区块矿工的奖励
const subsidy = 10

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

type TXInput struct {
	Txid      []byte
	Vout      int
	ScripsSig string
}

type TXOutput struct {
	Value        int
	ScriptPubKey string
}

func NewUTXOTransaction(from, to string, amount int, bc *Blockchain) *Transaction {
	var inputs []TXInput
	var outputs []TXOutput
	// acc 是输出的区块的value之和，validOutputs是map[string][]int
	// 保存的所有使用TXOutput的编号
	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		log.Panic("ERROR: Not enouth funds")
	}
	// slice的遍历顺序不确定，对应的是key/value
	for txid, outs := range validOutputs {
		// hex.DecodeString返回的是十六进制的[]byte
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}
		// out是当前选中的Transaction中选中的TXOutput的编号
		for _, out := range outs {
			// 将选中的上一个区块的输出作为这一个区块的输入
			input := TXInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}
	// 输出的值,目前锁定脚本from to都只是一个字符串
	outputs = append(outputs, TXOutput{amount, to})
	// 如果收集到的之前区块的输出已经大于本区块需要传输的数量
	// 就在本区块中添加一个TXOutput将多出来的传输给from
	// 一般情况下,这一步是要发生的
	if acc >= amount {
		outputs = append(outputs, TXOutput{acc - amount, from})
	}
	// 根据这些inputs, outputs生成新的Transaction
	tx := Transaction{nil, inputs, outputs}
	//
	tx.SetID()

	return &tx
}

// 被NewUTXOtransaction函数调用，传入的是address的值是from
func (bc *Blockchain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	unspentTXs := bc.FindUnspentTransactions(address)
	accumulated := 0
Work:
	// 循环每一个Transaction
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)

		// tx.Vout是一个Transaction中所有输出的切片，即[]TXOutput
		// outIdx是循环的编号，也可以说是这个交易中TXOutput的编号
		// out是其对应的一个输出
		for outIdx, out := range tx.Vout {
			// 只要需要的value还没够，就继续收集交易
			if out.CanBeUnlockedWith(address) && accumulated < amount {
				accumulated += out.Value
				// Transaction到保存数字切片的映射，
				// 数字切片里是当前Transaction里，被使用的TXOutput的编号
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOutputs
}

// coinbase对于矿工的奖励交易
func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}

	txin := TXInput{[]byte{}, -1, data}
	txout := TXOutput{subsidy, to}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{txout}}
	tx.SetID()

	return &tx
}

// 在前面的函数中调用的,设置新Transaction的ID
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

// 判断是否有权使用,在下面的函数中被调用，传入的是from字符串，
// 相等则表明这个TXInput是将coin这个值传给了from这个地址
func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool {
	return in.ScripsSig == unlockingData
}

func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool {
	return out.ScriptPubKey == unlockingData
}

// 收集包含未花费的输出的Transaction
func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
		Outputs:
			for outIdx, out := range tx.Vout {
				// 第一次执行到这里，这个if肯定是会跳过的
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				if out.CanBeUnlockedWith(address) {
					unspentTXs = append(unspentTXs, *tx)
				}
			}
			// 不是coinbase交易，
			if tx.IsCoinbase() == false {
				// 遍历当前Transaction的全部TXInput
				for _, in := range tx.Vin {
					// address是from的实参 ，这个if是为了判断这个in（类型为TXInput）
					// 是发送给address这个地址的，address这个地址是可以使用其中的coin的
					if in.CanUnlockOutputWith(address) {
						// 转换为16进制字符串
						inTxID := hex.EncodeToString(in.Txid)
						// 将可用的TXInput的Txid，及其对应的Vout对应起来
						spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
					}
				}
			}
		}
		// 已经遍历到了创世区块，即所有区块都被检查过了，可以结束了
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return unspentTXs
}

// 判断是否是coinbase交易
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func (bc *Blockchain) FindUTXO(address string) []TXOutput {
	var UTXOs []TXOutput
	unspentTransactions := bc.FindUnspentTransactions(address)

	for _, tx := range unspentTransactions {
		for _, out := range tx.Vout {
			if out.CanBeUnlockedWith(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}
	return UTXOs
}
