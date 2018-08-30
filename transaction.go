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

	// 余额不足，返回一个异常
	if acc < amount {
		log.Panic("ERROR: Not enouth funds")
	}
	// slice的遍历顺序不确定，对应的是key/value
	for txid, outs := range validOutputs {
		// hex.DecodeString将使用字符串表示的hash转化为十六进制的[]byte，
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}
		// out是当前选中的Transaction中选中的TXOutput的编号
		for _, out := range outs {
			// 引用选中的上一个区块的TXOutput输出作为这一个Transaction的输入
			// 记录被引用的Transaction的hash（即txID），具体的TXOutput的编号（即out），
			// from是拥有者的地址
			input := TXInput{txID, out, from}
			// 正在构建新的[]TXInput
			inputs = append(inputs, input)
		}
	}
	// 一个Transaction的[]TXOutput，切片有一个或两个元素
	// 一个是花出去。一个是找零
	outputs = append(outputs, TXOutput{amount, to})
	// 如果收集到的之前区块的输出已经大于本区块需要传输的数量
	// 就在本区块中添加一个TXOutput将多出来的传输给from，相当于找零
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
	// 循环每一个找到的每一个包含有发送给from且未使用的coin的Transaction
	for _, tx := range unspentTXs {
		// tx.ID保存的是这个tx的hash类型是[]byte，此处此处将其转化为字符串的形式
		txID := hex.EncodeToString(tx.ID)

		// tx.Vout是一个Transaction中所有输出的切片，即[]TXOutput
		// outIdx是编号，从0开始
		// out是一个TXOutput结构
		for outIdx, out := range tx.Vout {
			// 只要需要的amount还不足以支付费用，就继续收集交易
			if out.CanBeUnlockedWith(address) && accumulated < amount {
				accumulated += out.Value
				// 将这个Transaction的hash与其里面的[]TXOutput中能用的TXOutput编号组成的[]int对应
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				// 当找到的足够的TXOutput就跳出循环
				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	// 返回收集到的总的coin的值，这个值可能会比amount大一点，到时候会把多余的值找零给to地址，
	// 还有可能是遍历了所有的区块的所有Transaction的所有TXOutput，函数依然会返回，要怎么办交易调用它的函数
	// unsentOutputs中有收集到的Transaction的hash，每个hash对应一个[]int，这样叫可以找到具体的TXOutput
	return accumulated, unspentOutputs
}

// coinbase对于矿工的奖励交易
func NewCoinbaseTX(to, data string) *Transaction {
	// data可事先设置其内容
	// 若没有设置，则使用默认的设置，使用下面的字符串
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}
	// coinbaseTX的特点，TXInout是没有的，就设置成这样
	// 输出是把挖出一个区块的奖励发送给这个地址
	txin := TXInput{[]byte{}, -1, data}
	txout := TXOutput{subsidy, to}
	// 临时将此Transaction的ID设置为nil，[]TXInput,[]TXOutput中都只有一个元素
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{txout}}
	// 计算Transaction的hash，保存在[]ID中
	tx.SetID()

	return &tx
}

// 在前面的函数中调用的,设置新Transaction的ID
// 一个Transaction的ID即是一个tx的hash
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
	// 遍历每一个block
	for {
		block := bci.Next()
		// 遍历这个block的每个Transaction，他们保存在block的Transactions切片中
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
		Outputs:
			// 遍历当前Transaction的每个TXOutput
			for outIdx, out := range tx.Vout {
				// 第一次执行到这里，这个if肯定是会跳过的
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				// 若这个Transaction中有发送到address的TXOutput，
				// 就将这个Transaction地址保存到切片，
				if out.CanBeUnlockedWith(address) {
					// tx在此处是一个局部变量，但编译器会自动做逃逸分析
					// 简单来说，这样做是没错的，变量的作用域跑出了程序块的范围
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
	// 将找到的Transaction指针的切片返回，
	// 这些Transaction满足
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
