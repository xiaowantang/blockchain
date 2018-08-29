package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

type CLI struct {
	bc *Blockchain
}

const usage = `
Usage:
	createblockchain -address 创建一个新的区块链数据库,并创建创世区块,
		创世区块的奖励发给调用的address
	printchain 输出所有的区块的信息
	send -from -to -amount 从from地址发送amount数量的coin到to地址
	getblance -address 获取address的余额
`

func (cli *CLI) printUsage() {
	fmt.Println(usage)
}

// 当没有参数时，直接退出
func (cli *CLI) validateArges() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

// 解析命令行参数并调用相关函数
func (cli *CLI) Run() {
	cli.validateArges()

	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)

	createBlockchainAddress := createBlockchainCmd.String("address", "", "Block data")
	sendFrom := sendCmd.String("from", "", "Send from")
	sendTo := sendCmd.String("to", "", "Send to")
	sendAmount := sendCmd.Int("amount", 0, "Send Amount")
	getBalanceAddress := getBalanceCmd.String("address", "", "Address")

	switch os.Args[1] {
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		os.Exit(1)
	}
	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			os.Exit(1)
		}
		cli.createBlockchain(*createBlockchainAddress)
	}
	if printChainCmd.Parsed() {
		cli.printChain()
	}
	if sendCmd.Parsed() {
		if (*sendFrom == "") || (*sendTo == "") || (*sendFrom == "") {
			sendCmd.Usage()
			os.Exit(1)
		}
		cli.send(*sendFrom, *sendTo, *sendAmount)
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}
		cli.getBalance(*getBalanceAddress)
	}
}

func (cli *CLI) createBlockchain(address string) {
	bc := CreateBlockchain(address)
	defer bc.db.Close()
	fmt.Println("创建区块链成功!")
}

func (cli *CLI) printChain() {
	bc := NewBlockchain()
	defer bc.db.Close()
	bci := bc.Iterator()

	for {
		block := bci.Next()

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %x\n", block.HashTransactions())
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Pow: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		// 已经到了创世区块
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}

// 从from到to发送币
func (cli *CLI) send(from, to string, amount int) {
	bc := NewBlockchain()
	defer bc.db.Close()

	tx := NewUTXOTransaction(from, to, amount, bc)
	bc.MineBlock([]*Transaction{tx})
	fmt.Println("Success!")
}

// 输出from的余额
func (cli *CLI) getBalance(address string) {
	bc := NewBlockchain()
	defer bc.db.Close()

	balance := 0
	UTXOs := bc.FindUTXO(address)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
}
