package main

import (
	"fmt"
	"io/ioutil"
	"lxsoft/amwj/logx"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	if len(os.Args) < 3 {
		logx.Fatalln("Usage: bin.exe siteIndexTemplateFile MaxGoodsCountPerShopTag [debug]")
		os.Exit(1)
	}

	ok := updateSiteIndex()
	if !ok {
		logx.Fatalln("updateSiteIndex failed.")
		os.Exit(1)
	}
}

func updateSiteIndex() bool {

	//读取模板文件
	tplBytes, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		logx.Fatalln("Read template file failed.", err)
		return false
	}
	tplString := strings.ReplaceAll(string(tplBytes), "\r", "")

	//读取标签文件
	tagBytes, err := ioutil.ReadFile("./labels.txt")
	if err != nil {
		logx.Fatalln("Read labels.txt failed.", err)
		return false
	}
	tagList := strings.Split(string(tagBytes), "\n")

	//生成<li>HTML片段
	var snip string
	for _, tag := range tagList {
		tagText := strings.TrimSpace(tag)
		if len(tagText) > 0 {
			snip += fmt.Sprintf("<li class='tag'>%s</li>\n\t\t\t", tagText)
		}
	}

	//替换模版数据
	html := strings.Replace(tplString, "{{tagslist}}", snip, 1)

	//取得网站路径
	sitePath, _ := filepath.Split(os.Args[1])
	siteIndex := filepath.Join(sitePath, "index.html")

	//更新网站首页index.html
	err = ioutil.WriteFile(siteIndex, []byte(html), 0664)
	if err != nil {
		logx.Fatalln("Write index.html failed.", err)
		return false
	}

	return true
}
