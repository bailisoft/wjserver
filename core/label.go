package core

import (
	"bufio"
	"fmt"
	"io"
	"lxsoft/amwj/data"
	"lxsoft/amwj/logx"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	//主链
	chainMap sync.Map

	//每店同标签最多显示数量
	maxGoodsCountPerShopTag int
)

func init() {

	if len(os.Args) < 3 {
		logx.Fatalln("Usage: bin.exe siteIndexTemplateFile MaxGoodsCountPerShopTag [debug]")
		os.Exit(1)
	}

	v, err := strconv.Atoi(os.Args[2])
	if err != nil || v < 1 {
		logx.Fatalln("Usage: bin.exe siteIndexTemplateFile MaxGoodsCountPerShopTag [debug]")
		os.Exit(1)
	}
	maxGoodsCountPerShopTag = v

	err = loadLabelsAndChains()
	if err != nil {
		logx.Logf("loadLabelsAndChains failed: %v", err)
	}
}

func loadLabelsAndChains() error {

	tagFile, err := os.Open("./labels.txt")
	if err != nil {
		return err
	}

	reader := bufio.NewReader(tagFile)
	for {
		tag, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}

		tag = strings.TrimRight(tag, "\n")
		if len(tag) > 0 {
			//创建主链
			chain := createTagChain(tag)
			chainMap.Store(tag, chain)

			//加载数据
			filePath := fmt.Sprintf("./chains/%s.txt", tag)
			_, err = os.Lstat(filePath)
			if err == nil {
				chainFile, err := os.Open(filePath)
				if err != nil {
					tagFile.Close()
					return err
				}
				reader := bufio.NewReader(chainFile)
				for {
					str, err := reader.ReadString('\n')
					if err == io.EOF {
						break
					}
					flds := strings.Split(strings.TrimRight(str, "\n"), "\t")
					if len(flds) > 5 {
						operate := flds[1]
						geoKey, _ := strconv.ParseUint(flds[2], 10, 64)
						backer := flds[3]
						shop := flds[4]
						goods := strings.Join(flds[5:], "\t")
						if "P" == operate {
							//启动初始，不要加锁
							chain.putShopGoods(geoKey, backer, shop, goods)
						}
						if "R" == operate {
							//启动初始，不要加锁
							chain.removeShopNode(geoKey)
						}
					}
				}

				chainFile.Close()
			}
		}
	}

	tagFile.Close()

	return nil
}

//通知记录
func recordShopNodeRemoving(tag string, geokey uint64, backer string, shop string) {
	line := fmt.Sprintf("%s\tR\t%d\t%s\t%s\t", time.Now().Format("2006-01-02 15:04:05"),
		geokey, backer, shop) //分段符\t数量与添加记录一致
	data.ChainChan <- (tag + "\f" + line)
}

//仅用于前期后台直接使用 kill 发送信号更新新标签。后期使用web请求后废除
func CheckReloadNewLabels() {
	tagFile, err := os.Open("./labels.txt")
	if err != nil {
		return
	}

	reader := bufio.NewReader(tagFile)
	for {
		tag, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}

		_, found := chainMap.Load(tag)
		if !found {
			chain := createTagChain(tag)
			chainMap.Store(tag, chain)
		}
	}
}

//取链（如无新建）
func CheckGetChain(label string) *TagChain {

	var chain *TagChain

	v, found := chainMap.Load(label)
	if found {

		//使用现有链
		chain = v.(*TagChain)

	} else {

		//创建新链
		chain = createTagChain(label)
		chainMap.Store(label, chain)

		//通知记录
		data.LabelChan <- label
	}

	return chain
}

//同步库存变化
func SynStock(backer string, shop string, x int, y int, dels string, tags string, goods string) {

	//参数约定
	shopKey := HashGeokey(x, y)
	tagList := strings.Split(tags, "\n")
	goodsList := strings.Split(goods, "\n")
	if len(tagList) != len(goodsList) {
		return
	}

	//变无库存标签删除
	if dels == "*" {
		//特殊约定"*"表示后面tags为有库存的全部tags，因此全部删除
		removeShopFromAllChains(shopKey, backer, shop)
	} else {
		//后面tags不包含全部有库存的，只是有变动的，这里按指定删除
		delTags := strings.Split(dels, "\t")
		removShopFromSomeChains(delTags, shopKey, backer, shop)
	}

	//节点库存替换覆盖
	for i, tag := range tagList {
		//取链
		chain := CheckGetChain(tag) //函数内保证有链

		//上架
		chain.mutex.Lock()
		chain.putShopGoods(shopKey, backer, shop, goodsList[i])
		chain.mutex.Unlock()

		//通知记录
		line := fmt.Sprintf("%s\tP\t%d\t%s\t%s\t%s", time.Now().Format("2006-01-02 15:04:05"),
			shopKey, backer, shop, goodsList[i])
		data.ChainChan <- (tag + "\f" + line)
	}
}

//返回全部标签
func AllLabels() string {

	list := make([]string, 0)

	chainMap.Range(func(k, v interface{}) bool {
		list = append(list, k.(string))
		return true
	})

	return strings.Join(list, "\t")
}

//搜索货品
func SearchGoods(tag string, x int, y int, mores int) string {

	v, found := chainMap.Load(tag)
	if found {

		//坐标
		loc := HashGeokey(x, y)            //60位中后32位精度在百米直至厘米，范围搜索不需要，但还原经纬度时需要。
		tol := uint64(1 << (32 + 2*mores)) //geokey值为60位，(60-32)/2=14，因此mores必须小于14，参数检查须知。
		//fmt.Printf("http search %s, key:%d, tol:%d,  min:%d,  max:%d\n", tag, loc, tol, loc-tol, loc+tol)

		//用链
		var chain *TagChain = v.(*TagChain)
		chain.mutex.RLock()
		goods := chain.SearchGoods(loc, tol)
		chain.mutex.RUnlock()

		return goods
	}

	return "NoChain"
}
