package main

import (
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"github.com/dpooltop/ethpool/blocks"
	"github.com/dpooltop/ethpool/rpc"
	"github.com/dpooltop/ethpool/util"
)

type JobMaker struct {
	config *Config
	blockTemplate atomic.Value
	upstream int32
	upstreams []*rpc.RPCClient
	diff string
}

func NewJobMaker(cfg *Config) *JobMaker {
	jobMaker := &JobMaker{config: cfg}

	jobMaker.upstreams = make([]*rpc.RPCClient, len(cfg.Upstream))
	for i, v := range cfg.Upstream {
		jobMaker.upstreams[i] = rpc.NewRPCClient(v.Name, v.Url, v.Timeout)
		log.Printf("Upstream: %s => %s", v.Name, v.Url)
	}
	log.Printf("Default upstream: %s => %s", jobMaker.rpc().Name, jobMaker.rpc().Url)

	jobMaker.fetchBlockTemplate()
	return jobMaker
}

func (jobMaker *JobMaker) Start() {
	refreshIntv := util.MustParseDuration(cfg.BlockRefreshInterval)
	refreshTimer := time.NewTimer(refreshIntv)
	log.Printf("Set block refresh every %v", refreshIntv)

	checkIntv := util.MustParseDuration(cfg.UpstreamCheckInterval)
	checkTimer := time.NewTimer(checkIntv)
	
	go func() {
		for {
			select {
			case <-refreshTimer.C:
				jobMaker.fetchBlockTemplate()
				refreshTimer.Reset(refreshIntv)
			}
		}
	}()
	
	go func() {
		for {
			select {
			case <-checkTimer.C:
				jobMaker.checkUpstreams()
				checkTimer.Reset(checkIntv)
			}
		}
	}()
}

func (jobMaker *JobMaker) rpc() *rpc.RPCClient {
	i := atomic.LoadInt32(&jobMaker.upstream)
	return jobMaker.upstreams[i]
}

func (jobMaker *JobMaker) checkUpstreams() {
	candidate := int32(0)
	backup := false

	for i, v := range jobMaker.upstreams {
		if v.Check() && !backup {
			candidate = int32(i)
			backup = true
		}
	}

	if jobMaker.upstream != candidate {
		log.Printf("Switching to %v upstream", jobMaker.upstreams[candidate].Name)
		atomic.StoreInt32(&jobMaker.upstream, candidate)
	}
}

func (jobMaker *JobMaker) currentBlockTemplate() *blocks.BlockTemplate {
	t := jobMaker.blockTemplate.Load()
	if t != nil {
		return t.(*blocks.BlockTemplate)
	} else {
		return nil
	}
}

func (jobMaker *JobMaker) fetchBlockTemplate() {
	rpc := jobMaker.rpc()
	t := jobMaker.currentBlockTemplate()
	reply, err := rpc.GetWork()
	if err != nil {
		log.Printf("Error while refreshing block template on %s: %s", rpc.Name, err)
		return
	}
	// No need to update, we have fresh job
	if t != nil && t.Header == reply[0] {
		return
	}

	_, height, diff, err := jobMaker.fetchPendingBlock()
	if err != nil {
		log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
		return
	}

	newTemplate := blocks.BlockTemplate{
		Header:               reply[0],
		Seed:                 reply[1],
		Target:               reply[2],
		Height:               height,
		Difficulty:           big.NewInt(diff),
	}
	
	jobMaker.blockTemplate.Store(&newTemplate)
	log.Printf("New block to mine on %s at height %d / %s", rpc.Name, height, reply[0])
}

func (jobMaker *JobMaker) fetchPendingBlock() (*rpc.GetBlockReplyPart, uint64, int64, error) {
	rpc := jobMaker.rpc()
	reply, err := rpc.GetPendingBlock()
	if err != nil {
		log.Printf("Error while refreshing pending block on %s: %s", rpc.Name, err)
		return nil, 0, 0, err
	}
	blockNumber, err := strconv.ParseUint(strings.Replace(reply.Number, "0x", "", -1), 16, 64)
	if err != nil {
		log.Println("Can't parse pending block number")
		return nil, 0, 0, err
	}
	blockDiff, err := strconv.ParseInt(strings.Replace(reply.Difficulty, "0x", "", -1), 16, 64)
	if err != nil {
		log.Println("Can't parse pending block difficulty")
		return nil, 0, 0, err
	}
	return reply, blockNumber, blockDiff, nil
}
