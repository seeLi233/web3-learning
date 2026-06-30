package contracts

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

func work(id int, jobs <-chan int, result chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		fmt.Printf("worker %d 开始处理任务 %d\n", id, job)
		result <- job * 2 // 模拟任务计算
		fmt.Printf("worker %d 完成任务 %d\n", id, job)
	}
}

func fetchFirst() string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch1 := make(chan string)
	ch2 := make(chan string)
	ch3 := make(chan string)

	go func() {
		time.Sleep(300 * time.Millisecond)
		select {
		case ch1 <- "data-from-slow-source":
			fmt.Println("G1:发送成功，退出")
		case <-ctx.Done():
			fmt.Println("G1: 被取消，无需发送")
			return
		}
	}()

	go func() {
		time.Sleep(150 * time.Millisecond)
		select {
		case ch2 <- "data-from-fast-cache":
			fmt.Println("G2:发送成功，退出")
		case <-ctx.Done():
			fmt.Println("G2: 被取消，无需发送")
			return
		}
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		select {
		case ch3 <- "data-from-fast-cache":
			fmt.Println("G2:发送成功，退出")
		case <-ctx.Done():
			fmt.Println("G2: 被取消，无需发送")
			return
		}
	}()

	select {
	case result := <-ch1:
		fmt.Println("main 收到:", result)
	case result := <-ch2:
		fmt.Println("main 收到:", result)
	case result := <-ch3:
		fmt.Println("main 收到:", result)
	case <-time.After(500 * time.Millisecond):
		fmt.Println("time out")
	}

	return "done"
}

type shard struct {
	mu sync.RWMutex
	kv map[string]interface{}
}

type ShardedMap struct {
	shards []*shard
}

func NewShardMap(n int) *ShardedMap {
	sm := &ShardedMap{
		shards: make([]*shard, n),
	}

	for i := 0; i < n; i++ {
		sm.shards[i] = &shard{
			kv: make(map[string]interface{}),
		}
	}
	return sm
}

func (s *ShardedMap) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))

	idx := h.Sum32() % uint32(len(s.shards))
	return s.shards[idx]
}

func (sm *ShardedMap) Get(key string) (interface{}, bool) {
	s := sm.getShard(key)
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.kv[key]
	return val, ok
}

func (sm *ShardedMap) Set(key string, value interface{}) {
	s := sm.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kv[key] = value
}

func (sm *ShardedMap) Delete(key string) {
	s := sm.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.kv, key)
}

func main() {
	const workerNum = 3
	jobs := make(chan int, 5)
	result := make(chan int, 5)

	var wg sync.WaitGroup

	for i := 1; i <= workerNum; i++ {
		wg.Add(1)
		go work(i, jobs, result, &wg)
	}

	for j := 1; j <= 5; j++ {
		jobs <- j
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(result)
	}()

	for res := range result {
		fmt.Printf("收到任务结果 %d\n", res)
	}

	fmt.Println("所有任务处理完毕，main退出")
}
