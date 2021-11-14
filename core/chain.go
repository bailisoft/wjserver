package core

import (
	"fmt"
	"lxsoft/amwj/geo"
	"math/bits"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	max_skip_level = 32
	max_find_half  = 100
)

// 店节点
//【注意】必须保证 key 的唯一不重复！！！因此，店位置必须精确到小数点后6位（大约分米级）
type ShopNode struct {
	nexts  []*ShopNode
	prev   *ShopNode
	key    uint64 //shop's geohash
	backer string
	shop   string
	goods  string //simply format by \t split
}

// 标签链跳表
type TagChain struct {
	mutex      sync.RWMutex
	startNexts [max_skip_level]*ShopNode
	endNexts   [max_skip_level]*ShopNode
	topIndex   int
	nodeCount  int
	tagName    string
}

//因为仅仅在插入节点一处使用，不需要这里加锁
func (this *TagChain) generateLevel(limitLevel int) int {
	var x uint64 = rand.Uint64() & ((1 << uint(limitLevel-1)) - 1)
	zeroes := bits.TrailingZeros64(x)
	if zeroes <= limitLevel {
		return zeroes
	}
	return limitLevel - 1
}

//内部使用，不另外加锁
func (this *TagChain) locateEntryIndex(key uint64, deepIndex int) int {
	for i := this.topIndex; i >= 0; i-- {
		if this.startNexts[i] != nil && this.startNexts[i].key <= key || i <= deepIndex {
			return i
		}
	}
	return 0
}

//为打印记录随机种子可重现测试，设计两个init函数
func createTagChainBySeed(tag string, seed int64) *TagChain {

	rand.Seed(seed)
	//fmt.Println(tag, "seed:", seed)

	chain := TagChain{
		startNexts: [max_skip_level]*ShopNode{},
		endNexts:   [max_skip_level]*ShopNode{},
		topIndex:   0,
		nodeCount:  0,
		tagName:    tag,
	}

	return &chain
}

//生产使用
func createTagChain(tag string) *TagChain {
	return createTagChainBySeed(tag, time.Now().UTC().UnixNano())
}

// 查找店节点///////////////////////////////////////////////////////////////////////
// 注意：务必由调用处加锁。这里不加锁，是防止调用外面有锁，重复加锁，会锁死。
func (this *TagChain) findNodeBykey(key uint64, allowGreater bool, ensureSearch bool) (*ShopNode, bool) {

	//空链
	if this.startNexts[0] == nil {
		return nil, false
	}

	//起始层
	index := this.locateEntryIndex(key, 0)
	var currentNode *ShopNode = this.startNexts[index]
	if currentNode.key == key {
		return currentNode, true
	}

	//宽松条件
	if allowGreater && currentNode.key > key {
		return currentNode, true
	}

	//层历
	var foundNode *ShopNode = nil
	var foundOk bool = false
	var farNext *ShopNode
	for {
		farNext = currentNode.nexts[index]

		// 索引指向
		if farNext != nil && farNext.key <= key {

			// 向右
			currentNode = farNext
			if currentNode.key == key {
				foundNode = currentNode
				foundOk = true
				break
			}

		} else {

			if index > 0 {
				self := currentNode.nexts[0]
				if self != nil && self.key == key {
					foundNode = self
					foundOk = true
					break
				}

				// 向下
				index--

			} else {

				// 到底了
				if allowGreater {
					foundNode = farNext
					foundOk = (farNext != nil)
				}
				break
			}
		}
	}

	//搜索保证
	if ensureSearch && foundNode == nil {
		return this.startNexts[0], true
	}

	//定位结果
	return foundNode, foundOk
}

// 上架货品（整店货品数据完整替换）///////////////////////////////////////////////////
// 注意：务必由调用处加锁。这里不加锁，是防止调用外面有锁，重复加锁，会锁死。
func (this *TagChain) putShopGoods(key uint64, backer string, shop string, goods string) {

	//如果找到
	node, ok := this.findNodeBykey(key, false, false)
	if ok {
		node.goods = goods
		return
	}

	//准备节点数据
	deepIndex := this.generateLevel(max_skip_level)
	if deepIndex > this.topIndex {
		deepIndex = this.topIndex + 1
		this.topIndex = deepIndex
	}

	elem := &ShopNode{
		nexts:  make([]*ShopNode, deepIndex+1),
		key:    key,
		backer: backer,
		shop:   shop,
		goods:  goods,
	}

	//节点数更新
	this.nodeCount++

	//处理标志
	newFirst := true
	newLast := true
	if this.startNexts[0] != nil && this.endNexts[0] != nil {
		newFirst = elem.key < this.startNexts[0].key
		newLast = elem.key > this.endNexts[0].key
	}

	//非头非尾普通位置一般处理
	normallyInserting := false
	if !newFirst && !newLast {

		normallyInserting = true

		index := this.locateEntryIndex(elem.key, deepIndex)

		var currentNode *ShopNode
		nextNode := this.startNexts[index]

		for {

			if currentNode == nil {
				nextNode = this.startNexts[index]
			} else {
				nextNode = currentNode.nexts[index]
			}

			// Connect node to next
			if index <= deepIndex && (nextNode == nil || nextNode.key > elem.key) {

				elem.nexts[index] = nextNode
				if currentNode != nil {
					currentNode.nexts[index] = elem
				}
				if index == 0 {
					elem.prev = currentNode
					if nextNode != nil {
						nextNode.prev = elem
					}

				}
			}

			if nextNode != nil && nextNode.key <= elem.key {
				// Go right
				currentNode = nextNode
			} else {
				// Go down
				index--
				if index < 0 {
					break
				}
			}
		}
	}

	//全部情况逐层处理
	for i := deepIndex; i >= 0; i-- {

		done := false

		if newFirst || normallyInserting {

			//新节点的level有可能目前最高，此时t.startNexts[高处]==nil
			if this.startNexts[i] == nil || this.startNexts[i].key > elem.key {

				if i == 0 && this.startNexts[i] != nil {
					this.startNexts[i].prev = elem

				}

				elem.nexts[i] = this.startNexts[i] //赋值后仍然可能为nil
				this.startNexts[i] = elem

			}

			//根据刚刚条件赋值，推知以下表明新节点当前最高
			if elem.nexts[i] == nil {
				this.endNexts[i] = elem

			}

			done = true

		}

		if newLast {
			if !newFirst {
				if this.endNexts[i] != nil {
					this.endNexts[i].nexts[i] = elem

				}
				if i == 0 {
					elem.prev = this.endNexts[i]

				}
				this.endNexts[i] = elem

			}

			//新节点成为新尾部并且碰巧最高
			if this.startNexts[i] == nil {
				this.startNexts[i] = elem

			}

			done = true
		}

		if !done {
			break
		}

	}

	return
}

// 关店（删除整店节点）/////////////////////////////////////////////////////////////////////////////
// 注意：务必由调用处加锁。这里不加锁，是防止调用外面有锁，重复加锁，会锁死。
func (this *TagChain) removeShopNode(key uint64) bool {

	//空链
	if this.startNexts[0] == nil {
		return false
	}

	//开始层
	index := this.locateEntryIndex(key, 0)

	//查找
	var currentNode *ShopNode
	var nextNode *ShopNode
	var removed bool = false

	//层历
	for {

		if currentNode == nil {
			nextNode = this.startNexts[index]
		} else {
			nextNode = currentNode.nexts[index]
		}

		// 先判断是否找到
		if nextNode != nil && nextNode.key == key { //准确定位

			//本层跨连
			if currentNode != nil {
				currentNode.nexts[index] = nextNode.nexts[index]
			}

			//如果到达底层
			if index == 0 {
				if nextNode.nexts[0] != nil {
					nextNode.nexts[0].prev = currentNode
				}
				this.nodeCount--
			}

			//如果为本层头
			if this.startNexts[index] == nextNode {
				this.startNexts[index] = nextNode.nexts[index]
				if this.startNexts[index] == nil {
					this.topIndex = index - 1
				}
			}

			//如果为本层尾
			if nextNode.nexts[index] == nil {
				this.endNexts[index] = currentNode
			}

			//指针释放
			nextNode.nexts[index] = nil

			//结果
			removed = true

			//注意，必须层历完，这里不能结束
		}

		//如果不是，标志继续————不管是否在上层就已找到，仍然要遍历完各层
		if nextNode != nil && nextNode.key < key {

			//保持本层不变，向右找
			currentNode = nextNode

		} else {

			//降低一层，当前节点不变
			index--
			if index < 0 {
				break
			}
		}
	}

	//返回结果
	return removed
}

// 空链
func (this *TagChain) IsEmpty() bool {
	return this.startNexts[0] == nil
}

// 搜索（最终成果函数）///////////////////////////////////////////////////////////////
// 注意：务必由调用处加锁。这里不加锁，是防止调用外面有锁，重复加锁，会锁死。
func (this *TagChain) SearchGoods(loc uint64, tol uint64) string {

	//定位
	locateNode, ok := this.findNodeBykey(loc, true, true)
	if !ok {
		return "NoNode"
	}

	//先右爬历
	it := locateNode
	var finds []string = make([]string, 0, max_find_half)
	for it != nil && it.key <= loc+tol && len(finds) < max_find_half {
		goods := strings.Split(it.goods, "\t")
		if len(goods) > maxGoodsCountPerShopTag {
			goods = goods[:maxGoodsCountPerShopTag]
		}
		gs := strings.Join(goods, "\t")
		if len(goods) > 0 {
			lat, lng := geo.DecodeIntWithPrecision(it.key, 60)
			finds = append(finds, fmt.Sprintf("%s,%.6f,%.6f,%s", it.backer, lng, lat, gs))
		}
		it = it.nexts[0]
	}

	//再左爬历
	it = locateNode.prev
	var backwards []string = make([]string, 0, max_find_half)
	for it != nil && it.key >= loc-tol && len(backwards) < max_find_half {
		goods := strings.Split(it.goods, "\t")
		if len(goods) > maxGoodsCountPerShopTag {
			goods = goods[:maxGoodsCountPerShopTag]
		}
		gs := strings.Join(goods, "\t")
		if len(goods) > 0 {
			lat, lng := geo.DecodeIntWithPrecision(it.key, 60)
			backwards = append(backwards, fmt.Sprintf("%s,%.6f,%.6f,%s", it.backer, lng, lat, gs))
		}
		it = it.prev
	}

	if len(backwards) > 0 {
		finds = append(finds, backwards...)
	}

	return strings.Join(finds, "\n")
}

//仅仅测试用
func (this *TagChain) toString() string {
	var items []string
	n := 0
	node := this.startNexts[0]
	for node != nil {
		n++
		items = append(items, fmt.Sprintf("%d-%s-%s", node.key, node.shop, node.goods))
		node = node.nexts[0]
	}
	return strings.Join(items, "\n")
}

//仅仅调试使用
func (t *TagChain) chainStructString() string {
	s := fmt.Sprintf("%s chain topIndex:%d, startNexts len:%d, endNexts len:%d\n",
		t.tagName, t.topIndex, len(t.startNexts), len(t.endNexts))

	//startNexts
	for i, n := range t.startNexts {
		if n == nil {
			break
		}
		next := "---"
		if n != nil {
			next = n.shop
		}
		s += fmt.Sprintf("[%d]%s \t", i, next)
	}
	s += "\n"

	//nodes
	node := t.startNexts[0]
	for node != nil {

		s += fmt.Sprintf("(0)%d%s \t", node.key, node.shop)

		for i := 1; i <= t.topIndex; i++ {
			if i < len(node.nexts) {
				n := node.nexts[i]
				if n != nil {
					s += fmt.Sprintf("(%d)%s \t", i, n.shop)
				} else {
					s += fmt.Sprintf("(%d)--- \t", i)
				}
			}
		}
		s += "\n"
		node = node.nexts[0]
	}

	//endNexts
	for i, n := range t.endNexts {
		if n == nil {
			break
		}
		next := "---"
		if n != nil {
			next = n.shop
		}
		s += fmt.Sprintf("[%d]%s \t", i, next)
	}
	s += "\n"
	return s
}

//仅仅调试使用
func PrintChain(tag string) {
	v, found := chainMap.Load(tag)
	if found {
		chain, ok := v.(*TagChain)
		if ok {
			fmt.Println(chain.chainStructString())
		} else {
			fmt.Printf("%s链不存在\n", tag)
		}
	}
}
