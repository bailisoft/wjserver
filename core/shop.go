package core

import (
	"bufio"
	"fmt"
	"io"
	"lxsoft/amwj/data"
	"lxsoft/amwj/geo"
	"lxsoft/amwj/logx"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Shop struct {
	geokey uint64 //60位高精度geohash值，通常不会碰撞，实在碰撞提示用户微改坐标
	backer string //公司
	name   string //店名
	addr   string //地址
	tele   string //电话
	resp   string //店长
}

var shopMap sync.Map

func init() {

	err := loadShop()
	if err != nil {
		logx.Logln("loadShop failed: ", err)
	}

}

//调用处加锁
func loadShop() error {

	file, err := os.Open("./shops.txt")
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	for {
		str, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		flds := strings.Split(strings.TrimRight(str, "\n"), "\t")
		if len(flds) > 7 {
			operate := flds[1]
			k, _ := strconv.ParseUint(flds[2], 10, 60)
			if "I" == operate {
				var s Shop
				s.geokey = k
				s.backer = flds[3]
				s.name = flds[4]
				s.addr = flds[5]
				s.tele = flds[6]
				s.resp = flds[7]
				shopMap.Store(k, &s)
			} else {
				shopMap.Delete(k)
			}
		}
	}

	file.Close()

	return nil
}

//选用60位，是为了Decode时能够还原精度。
func HashGeokey(x int, y int) uint64 {
	return geo.EncodeIntWithPrecision(float64(y)/1000000.0, float64(x)/1000000.0, 60)
}

//未用到
func ParseGeokey(geokey uint64) (x int, y int) {
	lat, lng := geo.DecodeIntWithPrecision(geokey, 60)
	return int(math.Round(lng * 1000000)), int(math.Round(lat * 1000000))
}

//所有链删除门店节点
func removeShopFromAllChains(shopKey uint64, backer string, shop string) {
	//遍历全部标签链
	chainMap.Range(func(k, v interface{}) bool {
		tag := k.(string)

		//取链
		var chain *TagChain = v.(*TagChain)

		//删点
		chain.mutex.Lock()
		deleted := chain.removeShopNode(shopKey)
		chain.mutex.Unlock()

		//记录
		if deleted {
			recordShopNodeRemoving(tag, shopKey, backer, shop)
		}

		//继续遍历
		return true
	})
}

//删除某些链的门店节点
func removShopFromSomeChains(tags []string, shopKey uint64, backer string, shop string) {
	//遍历指定标签链
	for _, tag := range tags {

		c, found := chainMap.Load(tag)
		if found {

			//取链
			chain := c.(*TagChain)

			//删点
			chain.mutex.Lock()
			deleted := chain.removeShopNode(shopKey)
			chain.mutex.Unlock()

			//记录
			if deleted {
				recordShopNodeRemoving(tag, shopKey, backer, shop)
			}
		}
	}
}

//因为坐标涉及所有标签链节点键值，故而店不能更换坐标————除非关店重开。
func RegisterShop(backer string, name string, addr string, tele string, resp string, x int, y int) bool {

	//禁止坐标主键与其他公司门店坐标重复，但同公司可以。
	k := HashGeokey(x, y)
	s, ok := shopMap.Load(k)
	if ok {
		var shop *Shop = s.(*Shop)
		if shop.backer == backer {
			shop.name = name
			shop.addr = addr
			shop.tele = tele
			shop.resp = resp
			return true
		} else {
			return false
		}
	}

	//查授权店数
	backerMut.RLock()
	bk, ok := backerMap[backer]
	if !ok {
		backerMut.RUnlock()
		return false
	}
	limShops := bk.lics
	backerMut.RUnlock()

	//查同公司已上线店数
	puts := 0
	shopMap.Range(func(_, v interface{}) bool {
		var shop *Shop = v.(*Shop)
		if shop.backer == backer {
			puts++
		}
		return true
	})

	//超授权返回
	if puts >= limShops {
		return false
	}

	//新建
	var shop Shop
	shop.geokey = k
	shop.backer = backer
	shop.name = name
	shop.addr = addr
	shop.tele = tele
	shop.resp = resp
	shopMap.Store(k, &shop)

	//通知记录
	line := fmt.Sprintf("%s\tI\t%d\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n", time.Now().Format("2006-01-02 15:04:05"),
		k, backer, name, addr, tele, resp, x, y)
	data.ShopChan <- line

	return true
}

func UnRegisterShop(shopKey uint64, backer string, name string) bool {

	//检出
	s, ok := shopMap.Load(shopKey)
	if ok {
		shop := s.(*Shop)

		//验证自己公司
		if shop.backer == backer {

			//基本信息删除
			shopMap.Delete(shopKey)

			//删除各链节点
			removeShopFromAllChains(shopKey, backer, name)

			//通知记录
			line := fmt.Sprintf("%s\tD\t%d\t%s\t%s\t\t\t\t0\t0\n", time.Now().Format("2006-01-02 15:04:05"),
				shopKey, backer, name)
			data.ShopChan <- line

			return true
		}
	}

	return false
}

func ShopInfo(x int, y int, backer string) string {

	shopKey := HashGeokey(x, y)
	s, ok := shopMap.Load(shopKey)
	if ok {
		shop := s.(*Shop)
		if backer == shop.backer {
			return fmt.Sprintf("%s\t%s\t%s\t%s", shop.name, shop.addr, shop.tele, shop.resp)
		}
	}
	return ""
}
