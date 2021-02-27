package evm

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/okex/okexchain/x/evm/types"
)

var (
	goroutinePool chan struct{}
	globalWG      sync.WaitGroup
)

func initGoroutinePool() {
	goroutinePool = make(chan struct{}, (runtime.NumCPU()-1) * 64)
}

func addGoroutine() {
	goroutinePool <- struct{}{}
	globalWG.Add(1)
}

func finishGoroutine() {
	<- goroutinePool
	globalWG.Done()
}

// initExportEnv initials paths
func initExportEnv() {
	err := os.RemoveAll(absolutePath)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(absoluteCodePath, 0777)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(absoluteStoragePath, 0777)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(absoluteTxlogsFilePath, 0777)
	if err != nil {
		panic(err)
	}

	initGoroutinePool()
}

func createFile(filePath string) *os.File {
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	return file
}

func closeFile(writer *bufio.Writer, dstFile *os.File) {
	err := writer.Flush()
	if err != nil {
		panic(err)
	}
	err = dstFile.Close()
	if err != nil {
		panic(err)
	}
}

func writeOneLine(writer *bufio.Writer, data string) {
	_, err := writer.WriteString(data)
	if err != nil {
		panic(err)
	}
}

// syncWriteAccountCode writes types.Code into individual file in sync
func syncWriteAccountCode(ctx sdk.Context, k Keeper, address ethcmn.Address) {
	addGoroutine()
	defer finishGoroutine()

	code := k.GetCode(ctx, address)
	if len(code) != 0 {
		file := createFile(absoluteCodePath + address.String() + codeFileSuffix)
		writer := bufio.NewWriter(file)
		defer closeFile(writer, file)
		writeOneLine(writer, hexutil.Bytes(code).String())
	}
}

// syncWriteAccountStorageSlice writes types.Storage into individual file in sync
func syncWriteAccountStorageSlice(ctx sdk.Context, k Keeper, address ethcmn.Address) {
	addGoroutine()
	defer finishGoroutine()

	filename := absoluteStoragePath + address.String() + storageFileSuffix
	index := 0
	defer func() {
		if index == 0 {
			if err := os.Remove(filename); err != nil {
				panic(err)
			}
		}
	}()

	file := createFile(filename)
	writer := bufio.NewWriter(file)
	defer closeFile(writer, file)

	err := k.ForEachStorage(ctx, address, func(key, value ethcmn.Hash) bool {
		writeOneLine(writer, fmt.Sprintf("%s:%s\n", key.Hex(), value.Hex()))
		index++
		return false
	})
	if err != nil {
		panic(err)
	}
}

// writeAllTxLogs iterates all tx logs
func writeAllTxLogs(ctx sdk.Context, k Keeper) {
	k.IterateAllTxLogs(ctx, func(txLog types.TransactionLogs) (stop bool) {
		syncWriteTxLogs(txLog.Hash.String(), txLog.Logs)
		return false
	})
}

// syncWriteTxLogs writes []*ethtypes.Log based on one hash into individual file in sync
func syncWriteTxLogs(hash string, logs []*ethtypes.Log) {
	addGoroutine()
	defer finishGoroutine()

	dstFile := createFile(absoluteTxlogsFilePath + hash + txlogsFileSuffix)
	bufWriter := bufio.NewWriter(dstFile)
	defer closeFile(bufWriter, dstFile)

	data := types.ModuleCdc.MustMarshalJSON(logs)
	writeOneLine(bufWriter, string(data))
}

// readCodeFromFile used for setting types.Code into evm db when  InitGenesis
func readCodeFromFile(path string) []byte {
	bin, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	hexcode, err := hexutil.Decode(string(bin))
	if err != nil {
		panic(err)
	}

	return hexcode
}

// readStorageFromFile used for setting types.Storage into evm db when  InitGenesis
func readStorageFromFile(path string) types.Storage {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var states types.Storage
	rd := bufio.NewReader(f)
	for {
		kvStr, err := rd.ReadString('\n')
		if err != nil || io.EOF == err {
			break
		}
		// remove '\n', then split kvStr based on ':'
		kvPair := strings.Split(strings.ReplaceAll(kvStr, "\n", ""), ":")
		//convert hexStr into common.Hash struct
		k, v := ethcmn.HexToHash(kvPair[0]), ethcmn.HexToHash(kvPair[1])
		states = append(states, types.NewState(k, v))
	}
	return states
}

// readTxLogsFromFile used for setting []*ethtypes.Log into evm db when  InitGenesis
func readTxLogsFromFile(path string) []*ethtypes.Log {
	bin, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var txLogs []*ethtypes.Log
	types.ModuleCdc.MustUnmarshalJSON(bin, &txLogs)

	return txLogs
}

// convertHexStrToHash converts hexStr into ethcmn.Hash struct
func convertHexStrToHash(filename string) ethcmn.Hash {
	f := strings.Split(filename, ".") // make 0x0de69dd3828f8a79d6e51ae7eeb69a2b5f2.json -> [0x0de69dd3828f8a79d6e51ae7eeb69a2b5f2, json]
	hashStr := f[0]
	return ethcmn.HexToHash(hashStr)
}

// fileExist used for judging the file or path exist or not when InitGenesis
func pathExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}