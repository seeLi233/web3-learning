package demo

import (
	"hash/fnv"
)

// 简化版：一个桶，模仿bmap，每个桶存4组KV（简化8为4）
type bucket struct {
	keys [4]string
	vals [4]int
	used int     // 当前占用数量
	next *bucket // 溢出链表
}

// 简化版hmap主体
type SimpleMap struct {
	buckets []*bucket
	size    int // 总元素个数
	bCnt    int // 桶数组长度
}

// 新建Map，指定初始桶数量
func NewSimpleMap(bucketCout int) *SimpleMap {
	buckets := make([]*bucket, bucketCout)
	for i := range buckets {
		buckets[i] = &bucket{}
	}
	return &SimpleMap{
		buckets: buckets,
		bCnt:    bucketCout,
	}
}

// 简易哈希函数：string key -> 桶下标
func (m *SimpleMap) hashIndex(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	hashVal := h.Sum32()
	return int(hashVal) % m.bCnt
}

// Put 插入/更新
func (m *SimpleMap) Put(key string, val int) {
	idx := m.hashIndex(key)
	cur := m.buckets[idx]

	// 1. 遍历整条桶链表, 看 key 是否已存在
	for cur != nil {
		for i := 0; i < cur.used; i++ {
			if cur.keys[i] == key {
				// 覆盖更新
				cur.vals[i] = val
				return
			}
		}
		cur = cur.next
	}

	// 2. 不存在，从头重新找空位写入
	cur = m.buckets[idx]
	for {
		if cur.used < 4 {
			// 当前桶有空位
			cur.keys[cur.used] = key
			cur.vals[cur.used] = val
			cur.used++
			m.size++
		}
		// 当前桶满，看下一个溢出桶
		if cur.next == nil {
			// 新建溢出桶
			cur.next = &bucket{}
		}
		cur = cur.next
	}
}

// Get 查询
func (m *SimpleMap) Get(key string) (int, bool) {
	idx := m.hashIndex(key)
	cur := m.buckets[idx]

	for cur != nil {
		for i := 0; i < cur.used; i++ {
			if cur.keys[i] == key {
				return cur.vals[i], true
			}
		}
		cur = cur.next
	}
	return 0, false
}

// Delete 删除
func (m *SimpleMap) Delete(key string) bool {
	idx := m.hashIndex(key)
	cur := m.buckets[idx]

	for cur != nil {
		for i := 0; i < cur.used; i++ {
			if cur.keys[i] == key {
				// 把最后一个元素挪过来覆盖当前位置（填充空位）
				lastIdx := cur.used - 1
				cur.keys[i] = cur.keys[lastIdx]
				cur.vals[i] = cur.vals[lastIdx]
				// 清空最后一位
				cur.keys[lastIdx] = ""
				cur.vals[lastIdx] = 0
				cur.used--
				m.size--
				return true
			}
		}
		cur = cur.next
	}
	return false
}

// func main() {
// 	// 初始化3个桶
// 	m := NewSimpleMap(3)

// 	// 插入
// 	m.Put("a", 1)
// 	m.Put("b", 2)
// 	m.Put("c", 3)
// 	m.Put("d", 4)
// 	m.Put("e", 5)
// 	m.Put("f", 6)
// 	m.Put("g", 7)
// 	fmt.Println("总元素数:", m.size)

// 	// 查询
// 	if v, ok := m.Get("d"); ok {
// 		fmt.Println("get d =", v)
// 	}

// 	// 更新
// 	m.Put("d", 99)
// 	v, _ := m.Get("d")
// 	fmt.Println("update d =", v)

// 	// 删除
// 	m.Delete("c")
// 	fmt.Println("删除c后总数:", m.size)
// 	if _, ok := m.Get("c"); !ok {
// 		fmt.Println("c已不存在")
// 	}
// }
