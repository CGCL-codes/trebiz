package main

import (
	"encoding/hex"
	"fmt"
	"github.com/spf13/viper"
	"github.com/treble-h/trebiz/sign"
	"math/rand"
	"sort"
	"strconv"
	"time"
)

func judgeNodeType(i int, b []int) bool {
	for _, v := range b {
		if i == v {
			return true
		}
	}
	return false
}

func generateRandomNumber(start int, end int, count int) []int {

	if end < start || (end-start) < count {
		return nil
	}

	nums := make([]int, 0)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(nums) < count {

		num := r.Intn((end - start)) + start

		exist := false
		for _, v := range nums {
			if v == num {
				exist = true
				break
			}
		}

		if !exist {
			nums = append(nums, num)
		}
	}
	return nums
}

func main() {

	ProcessCount := 1

	viperRead := viper.New()

	viperRead.SetConfigName("config_template_local") // name of config file (without extension)
	viperRead.AddConfigPath("./config_gen")          // path to look for the config file in
	err := viperRead.ReadInConfig()                  // Find and read the config file
	if err != nil {                                  // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	clusterInnerAddr := viperRead.GetStringMap("ip1s")

	//IP分配
	tempClusterMapInterface := viperRead.GetStringMap("ip2s")
	clusterMapInterface := make(map[string]string)
	for name, addr := range tempClusterMapInterface {
		rs := []rune(name)
		ipIndex, _ := strconv.Atoi(string(rs[4:]))
		if addrAsString, ok := addr.(string); ok {

			for j := 0; j < ProcessCount; j++ {
				if ipIndex == 0 {
					suScript := strconv.Itoa(0)
					clusterMapInterface["node"+suScript] = addrAsString
					break
				}
				suScript := strconv.Itoa((ipIndex-1)*ProcessCount + j + 1)
				clusterMapInterface["node"+suScript] = addrAsString
			}

		} else {
			panic("cluster in the config file cannot be decoded correctly")
		}
	}

	nodeNumber := len(clusterInnerAddr)
	clusterMapString := make(map[string]string, nodeNumber)

	clusterName := make([]string, nodeNumber)
	sort.Strings(clusterName)
	i := 0
	for name, addr := range clusterInnerAddr {
		if addrAsString, ok := addr.(string); ok {
			clusterMapString[name] = addrAsString
			clusterName[i] = name
			i++
		} else {
			panic("cluster in the config file cannot be decoded correctly")
		}
	}

	tempP2pPortMapInterface := viperRead.GetStringMap("peers_p2p_port")
	if nodeNumber != len(tempP2pPortMapInterface) {
		panic("p2p_listen_port does not match with cluster")
	}
	//处理监听端口，同一个Ip下监听端口不一样
	p2pPortMapInterface := make(map[string]int)

	//获得ip对应的端口，配置文件中只给出单个ip对应的name
	mapNameToP2PPort := make(map[string]int, nodeNumber)
	for name, _ := range clusterMapString {
		portAsInterface, ok := tempP2pPortMapInterface[name]
		if !ok {
			panic("p2p_listen_port does not match with cluster")
		}
		if portAsInt, ok := portAsInterface.(int); ok {
			//单机器起始端口
			mapNameToP2PPort[name] = portAsInt
			rs := []rune(name)
			ipIndex, _ := strconv.Atoi(string(rs[4:]))
			for j := 0; j < ProcessCount; j++ {
				if ipIndex == 0 {
					subScript := strconv.Itoa(0)
					p2pPortMapInterface["node"+subScript] = portAsInt + j*10
					break
				}
				subScript := strconv.Itoa((ipIndex-1)*ProcessCount + j + 1)
				p2pPortMapInterface["node"+subScript] = portAsInt + j*10
			}

		} else {
			panic("p2p_listen_port contains a non-int value")
		}
	}

	//generate ed keys,map name to key
	privateKeysRsa := make(map[string]string)
	publicKeysRsa := make(map[string]string)

	//生成49个公私钥对
	for i := 0; i < nodeNumber; i++ {
		for j := 0; j < ProcessCount; j++ {
			privateKey, publicKey, err := sign.GenKeys()
			if err != nil {
				panic(err)
			}
			if i == 0 {
				subScript := strconv.Itoa(0)
				publicKeysRsa["node"+subScript] = hex.EncodeToString(publicKey)
				privateKeysRsa["node"+subScript] = hex.EncodeToString(privateKey)
				break
			}
			subScript := strconv.Itoa((i-1)*ProcessCount + j + 1)
			publicKeysRsa["node"+subScript] = hex.EncodeToString(publicKey)
			privateKeysRsa["node"+subScript] = hex.EncodeToString(privateKey)
		}
	}

	//generate threshold keys
	TotalNodeNum := (nodeNumber-1)*ProcessCount + 1
	numT := TotalNodeNum - TotalNodeNum/3
	shares, pubPoly := sign.GenTSKeys(numT, TotalNodeNum)

	bgm := viperRead.GetInt("bgnum")
	abm := viperRead.GetInt("abmnum")
	pbm := viperRead.GetInt("pbmnum")
	fastNum := TotalNodeNum - abm - pbm/2
	//fastNum := 2
	fastShares, fastPubPoly := sign.GenTSKeys(fastNum, TotalNodeNum)

	rpcListenPort := viperRead.GetInt("rpc_listen_port")

	evilNode := generateRandomNumber(1, TotalNodeNum, bgm+abm+pbm)

	fmt.Println("EVILNODES", evilNode)

	bgnodes := evilNode[0:bgm]
	abmnodes := evilNode[bgm : bgm+abm]
	pbmodes := evilNode[bgm+abm:]

	fmt.Println("bgnodes", bgnodes)
	fmt.Println("abmnodes", abmnodes)
	fmt.Println("pbmodes", pbmodes)

	for _, name := range clusterName {
		fmt.Printf("sssss")
		viperWrite := viper.New()
		for j := 0; j < ProcessCount; j++ {
			index := strconv.Itoa(j)

			rs := []rune(name)

			ipIndex, err := strconv.Atoi(string(rs[4:]))

			if err != nil {
				panic("get replicaid failed")
			}

			var replicaId int

			if ipIndex == 0 {
				replicaId = 0
			} else {
				//计算节点下标
				replicaId = (ipIndex-1)*ProcessCount + j + 1
			}

			viperWrite.SetConfigFile(fmt.Sprintf("%s_%s.yaml", name, index))

			shareAsBytes, err := sign.EncodeTSPartialKey(shares[replicaId])
			if err != nil {
				panic("encode the share")
			}

			tsPubKeyAsBytes, err := sign.EncodeTSPublicKey(pubPoly)
			if err != nil {
				panic("encode the share")
			}

			fastShareAsBytes, err := sign.EncodeTSPartialKey(fastShares[replicaId])
			if err != nil {
				panic("encode the share")
			}

			fastTsPubKeyAsBytes, err := sign.EncodeTSPublicKey(fastPubPoly)
			if err != nil {
				panic("encode the share")
			}

			viperWrite.Set("name", "node"+strconv.Itoa(replicaId))
			viperWrite.Set("replicaId", replicaId)

			//同一个ip下节点共用同一个地址
			viperWrite.Set("address", clusterMapString[name])

			//同一个ip下进程监听端口不一样
			viperWrite.Set("p2p_listen_port", mapNameToP2PPort[name]+j*10)

			viperWrite.Set("peers_p2p_port", p2pPortMapInterface)

			//同一个ip下进程监听端口不一样
			viperWrite.Set("rpc_listen_port", rpcListenPort+j)

			viperWrite.Set("cluster_ips", clusterMapInterface)

			//分发公私钥
			viperWrite.Set("ed_prikey", privateKeysRsa["node"+strconv.Itoa(replicaId)])
			viperWrite.Set("cluster_ed_pubkey", publicKeysRsa)

			viperWrite.Set("tsShare", hex.EncodeToString(shareAsBytes))
			viperWrite.Set("tsPubKey", hex.EncodeToString(tsPubKeyAsBytes))

			viperWrite.Set("fasttsShare", hex.EncodeToString(fastShareAsBytes))
			viperWrite.Set("fasttsPubKey", hex.EncodeToString(fastTsPubKeyAsBytes))

			viperWrite.Set("batchtimeout", viperRead.GetInt("batchtimeout"))
			viperWrite.Set("viewchangetimeout", viperRead.GetInt("viewchangetimeout"))
			viperWrite.Set("batchsize", viperRead.GetInt("batchsize"))
			viperWrite.Set("checkPoint_t", viperRead.GetInt("checkPoint_t"))
			viperWrite.Set("log_k", viperRead.GetInt("log_k"))
			viperWrite.Set("maxpool", viperRead.GetInt("maxpool"))

			viperWrite.Set("fastpathtimeout", viperRead.GetInt("fast_path_timeout"))
			viperWrite.Set("sameiptimeout", viperRead.GetInt("sameiptimeout"))

			//viperWrite.Set("nodetype", viperRead.GetInt("fast_path_timeout"))
			viperWrite.Set("evilpr", viperRead.GetInt("evilpr"))
			viperWrite.Set("fastqcquorum", fastNum)

			if judgeNodeType(replicaId, bgnodes) {
				viperWrite.Set("nodetype", 1)
			} else if judgeNodeType(replicaId, abmnodes) {
				viperWrite.Set("nodetype", 2)
			} else if judgeNodeType(replicaId, pbmodes) {
				viperWrite.Set("nodetype", 3)
			} else {
				viperWrite.Set("nodetype", 0)
			}

			err = viperWrite.WriteConfig()
			if err != nil {
				return
			}
			if ipIndex == 0 {
				break
			}
		}
	}
}
